package web

import (
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	"ipamthing/internal/auth"
)

const (
	maxScanIntervalSec = 24 * 3600
	maxUsernameLen     = 64
	maxSubnetNameLen   = 100
	maxHostnameLen     = 253
	maxCommentsLen     = 2000
	maxTagsLen         = 512
)

// truncate caps s at max bytes. If the cut lands mid-rune the trailing
// invalid bytes are dropped via strings.ToValidUTF8 so the result is
// always a valid UTF-8 string.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return strings.ToValidUTF8(s[:max], "")
}

// clientIP returns the request's client IP, honoring X-Forwarded-For only
// when trustProxy is true. The rightmost XFF entry is used because a
// standard nginx "proxy_add_x_forwarded_for" directive appends the IP it
// observed as its immediate source — which is the real client IP. The
// leftmost entries may be attacker-supplied and must not be trusted.
func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if i := strings.LastIndexByte(xff, ','); i >= 0 {
				return strings.TrimSpace(xff[i+1:])
			}
			return strings.TrimSpace(xff)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// isHTTPS reports whether the request is being served over TLS, either
// directly or via a trusted reverse proxy.
func isHTTPS(r *http.Request, trustProxy bool) bool {
	if r.TLS != nil {
		return true
	}
	if trustProxy && r.Header.Get("X-Forwarded-Proto") == "https" {
		return true
	}
	return false
}

// safeRedirect returns target if it's a safe same-origin path, otherwise fallback.
// Accepts only paths that start with "/" and don't start with "//" or "/\"
// (both of which can be interpreted as host-relative URLs by browsers).
func safeRedirect(target, fallback string) string {
	if target == "" {
		return fallback
	}
	if !strings.HasPrefix(target, "/") {
		return fallback
	}
	if strings.HasPrefix(target, "//") || strings.HasPrefix(target, `/\`) {
		return fallback
	}
	return target
}

type userPrefs struct {
	Theme  string
	Layout string
}

type pageData struct {
	Title  string
	Flash  string
	Notice string
	Data   any
	Prefs  userPrefs
	CSRF   string
}

// readPrefs loads UI preferences for the authenticated user from the DB.
// Unauthenticated requests (e.g. login/setup pages) get defaults so the
// preference is a per-account property that follows the user across
// browsers and machines.
func (s *Server) readPrefs(r *http.Request) userPrefs {
	p := userPrefs{Theme: "light", Layout: "normal"}
	uid, ok := r.Context().Value(userIDKey).(int64)
	if !ok || uid == 0 {
		return p
	}
	var theme, layout string
	err := s.db.QueryRow(
		`SELECT ui_theme, ui_layout FROM users WHERE id = ?`, uid,
	).Scan(&theme, &layout)
	if err != nil {
		return p
	}
	if theme == "dark" {
		p.Theme = "dark"
	}
	if layout == "wide" {
		p.Layout = "wide"
	}
	return p
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, page string, data pageData) {
	t, ok := s.tmpls[page]
	if !ok {
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}
	data.Prefs = s.readPrefs(r)
	data.CSRF = s.ensureCSRFCookie(w, r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		s.logger.Error("render", "page", page, "err", err)
	}
}

// ensureCSRFCookie returns the client's csrf cookie value, issuing a fresh
// random one (and setting it on the response) if missing.
func (s *Server) ensureCSRFCookie(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie("csrf"); err == nil && len(c.Value) >= 32 {
		return c.Value
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		s.logger.Error("csrf rand", "err", err)
		return ""
	}
	token := hex.EncodeToString(buf)
	http.SetCookie(w, &http.Cookie{
		Name:     "csrf",
		Value:    token,
		Path:     "/",
		MaxAge:   365 * 24 * 3600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isHTTPS(r, s.trustProxy),
	})
	return token
}

func (s *Server) userCount() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// ---- auth flow ----

func (s *Server) getSetup(w http.ResponseWriter, r *http.Request) {
	if n, _ := s.userCount(); n > 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	s.render(w, r, "setup", pageData{Title: "Set up admin"})
}

func (s *Server) postSetup(w http.ResponseWriter, r *http.Request) {
	// Hold setupMu across the entire check → compare → insert → clear
	// so that two concurrent requests with the correct token can't both
	// see users=0 and end up creating two admin rows.
	s.setupMu.Lock()
	defer s.setupMu.Unlock()

	if n, _ := s.userCount(); n > 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	expected := s.setupToken
	submitted := r.FormValue("setup_token")
	if expected == "" || subtle.ConstantTimeCompare([]byte(submitted), []byte(expected)) != 1 {
		s.render(w, r, "setup", pageData{Title: "Set up admin", Flash: "invalid setup token — check the server logs or restart with -setup-token"})
		return
	}

	username := truncate(r.FormValue("username"), maxUsernameLen)
	password := r.FormValue("password")
	if username == "" || len(password) < 8 {
		s.render(w, r, "setup", pageData{Title: "Set up admin", Flash: "username required and password must be 8+ chars"})
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		http.Error(w, "hash error", http.StatusInternalServerError)
		return
	}
	if _, err := s.db.Exec(
		`INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)`,
		username, hash, time.Now().Unix(),
	); err != nil {
		s.render(w, r, "setup", pageData{Title: "Set up admin", Flash: "failed to create user"})
		return
	}

	s.setupToken = ""
	s.logger.Info("first-run setup completed; setup token consumed")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) getLogin(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "login", pageData{Title: "Sign in"})
}

func (s *Server) postLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r, s.trustProxy)
	if allowed, retryAfter := s.loginLimiter.Allow(ip); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
		flash := fmt.Sprintf("too many failed attempts — try again in %s", retryAfter.Round(time.Second))
		s.render(w, r, "login", pageData{Title: "Sign in", Flash: flash})
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	var id int64
	var hash string
	err := s.db.QueryRow(
		`SELECT id, password_hash FROM users WHERE username = ?`, username,
	).Scan(&id, &hash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	userFound := err == nil
	if !userFound {
		hash = auth.DummyHash()
	}
	// Always run bcrypt so timing does not leak user existence.
	passwordOK := auth.CheckPassword(hash, password)
	if !userFound || !passwordOK {
		s.loginLimiter.Fail(ip)
		s.render(w, r, "login", pageData{Title: "Sign in", Flash: "invalid credentials"})
		return
	}
	s.loginLimiter.Success(ip)

	sess, err := s.sessions.Create(id)
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	// Rotate: invalidate every other session for this user so a fresh
	// login logs them out of other browsers/devices.
	if err := s.sessions.DeleteOthers(id, sess.Token); err != nil {
		s.logger.Warn("session rotation", "user_id", id, "err", err)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    sess.Token,
		Path:     "/",
		Expires:  sess.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isHTTPS(r, s.trustProxy),
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) postLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(auth.SessionCookieName); err == nil {
		_ = s.sessions.Delete(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: auth.SessionCookieName, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) getIndex(w http.ResponseWriter, r *http.Request) {
	s.dashboard(w, r)
}

type dashboardTotals struct {
	Subnets   int
	Hosts     int
	Up        int
	Down      int
	Events24h int
}

type dashboardSubnet struct {
	ID              int64
	CIDR            string
	Name            string
	Hosts           int
	Up              int
	ScanEnabled     bool
	ScanIntervalSec int
	LastScanAt      int64
	LastScanErr     string
}

type dashboardPageData struct {
	Totals        dashboardTotals
	Subnets       []dashboardSubnet
	RecentEvents  []eventFeedRow
	ICMPAvailable bool
	ICMPMode      string
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	var d dashboardPageData
	d.ICMPAvailable = s.scanner.ICMPAvailable()

	since := time.Now().Add(-24 * time.Hour).Unix()
	if err := s.db.QueryRow(`
		SELECT (SELECT COUNT(*) FROM subnets),
		       (SELECT COUNT(*) FROM hosts),
		       (SELECT COUNT(*) FROM hosts WHERE status='up'),
		       (SELECT COUNT(*) FROM hosts WHERE status='down'),
		       (SELECT COUNT(*) FROM scan_events WHERE ts >= ?)
	`, since).Scan(&d.Totals.Subnets, &d.Totals.Hosts, &d.Totals.Up, &d.Totals.Down, &d.Totals.Events24h); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	srows, err := s.db.Query(`
		SELECT sn.id, sn.cidr, sn.name, sn.scan_enabled, sn.scan_interval_sec,
		       COALESCE(sn.last_scan_at, 0), sn.last_scan_err,
		       COUNT(h.id),
		       COALESCE(SUM(CASE WHEN h.status='up' THEN 1 ELSE 0 END), 0)
		FROM subnets sn
		LEFT JOIN hosts h ON h.subnet_id = sn.id
		GROUP BY sn.id ORDER BY sn.cidr
	`)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	for srows.Next() {
		var sr dashboardSubnet
		var enabled int
		if err := srows.Scan(&sr.ID, &sr.CIDR, &sr.Name, &enabled, &sr.ScanIntervalSec,
			&sr.LastScanAt, &sr.LastScanErr, &sr.Hosts, &sr.Up); err != nil {
			srows.Close()
			http.Error(w, "scan error", http.StatusInternalServerError)
			return
		}
		sr.ScanEnabled = enabled != 0
		d.Subnets = append(d.Subnets, sr)
	}
	srows.Close()

	erows, err := s.db.Query(`
		SELECT e.id, h.id, h.subnet_id, sn.cidr, h.ip,
		       h.hostname_override, h.hostname_dns, e.ts, e.event, e.details
		FROM scan_events e
		JOIN hosts h   ON e.host_id   = h.id
		JOIN subnets sn ON h.subnet_id = sn.id
		ORDER BY e.id DESC LIMIT 10
	`)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	for erows.Next() {
		var e eventFeedRow
		var override, dns string
		if err := erows.Scan(&e.EventID, &e.HostID, &e.SubnetID, &e.SubnetCIDR, &e.IP,
			&override, &dns, &e.TS, &e.Event, &e.Details); err != nil {
			erows.Close()
			http.Error(w, "scan error", http.StatusInternalServerError)
			return
		}
		if override != "" {
			e.EffectiveHostname = override
		} else {
			e.EffectiveHostname = dns
		}
		d.RecentEvents = append(d.RecentEvents, e)
	}
	erows.Close()

	s.render(w, r, "dashboard", pageData{Title: "Dashboard", Data: d})
}

// ---- subnets ----

type subnetRow struct {
	ID              int64
	CIDR            string
	Name            string
	ScanEnabled     bool
	ScanIntervalSec int
	LastScanAt      int64
	LastScanErr     string
}

func (s *Server) listSubnets(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(
		`SELECT id, cidr, name, scan_enabled, scan_interval_sec, COALESCE(last_scan_at, 0), last_scan_err
		 FROM subnets ORDER BY cidr`,
	)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var subnets []subnetRow
	for rows.Next() {
		var sr subnetRow
		var enabled int
		if err := rows.Scan(&sr.ID, &sr.CIDR, &sr.Name, &enabled, &sr.ScanIntervalSec, &sr.LastScanAt, &sr.LastScanErr); err != nil {
			http.Error(w, "scan error", http.StatusInternalServerError)
			return
		}
		sr.ScanEnabled = enabled != 0
		subnets = append(subnets, sr)
	}

	flash := ""
	switch r.URL.Query().Get("err") {
	case "invalid_cidr":
		flash = "invalid CIDR"
	case "insert_failed":
		flash = "failed to save subnet (duplicate CIDR?)"
	}
	s.render(w, r, "subnets", pageData{Title: "Subnets", Flash: flash, Data: subnets})
}

func (s *Server) createSubnet(w http.ResponseWriter, r *http.Request) {
	cidr := r.FormValue("cidr")
	name := truncate(r.FormValue("name"), maxSubnetNameLen)
	interval, _ := strconv.Atoi(r.FormValue("scan_interval_sec"))
	if interval < 30 {
		interval = 300
	}
	if interval > maxScanIntervalSec {
		interval = maxScanIntervalSec
	}
	if _, _, err := net.ParseCIDR(cidr); err != nil {
		http.Redirect(w, r, "/subnets?err=invalid_cidr", http.StatusSeeOther)
		return
	}
	if _, err := s.db.Exec(
		`INSERT INTO subnets (cidr, name, scan_enabled, scan_interval_sec, created_at) VALUES (?, ?, 1, ?, ?)`,
		cidr, name, interval, time.Now().Unix(),
	); err != nil {
		http.Redirect(w, r, "/subnets?err=insert_failed", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/subnets", http.StatusSeeOther)
}

func (s *Server) deleteSubnet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.db.Exec(`DELETE FROM subnets WHERE id = ?`, id); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/subnets", http.StatusSeeOther)
}

func (s *Server) editSubnet(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	name := truncate(r.FormValue("name"), maxSubnetNameLen)
	interval, _ := strconv.Atoi(r.FormValue("scan_interval_sec"))
	if interval < 30 {
		interval = 30
	}
	if interval > maxScanIntervalSec {
		interval = maxScanIntervalSec
	}
	if _, err := s.db.Exec(
		`UPDATE subnets SET name = ?, scan_interval_sec = ? WHERE id = ?`,
		name, interval, id,
	); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/subnets/%d", id), http.StatusSeeOther)
}

func (s *Server) toggleSubnet(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if _, err := s.db.Exec(`UPDATE subnets SET scan_enabled = 1 - scan_enabled WHERE id = ?`, id); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, safeRedirect(r.FormValue("redirect"), "/subnets"), http.StatusSeeOther)
}

func (s *Server) scanSubnet(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.scheduler.Trigger(id)
	http.Redirect(w, r, fmt.Sprintf("/subnets/%d?scan=queued", id), http.StatusSeeOther)
}

// ---- hosts ----

type hostView struct {
	ID               int64
	SubnetID         int64
	IP               string
	MAC              string
	HostnameDNS      string
	HostnameOverride string
	Status           string
	FirstSeen        int64
	LastSeen         int64
	Comments         string
	Tags             string
}

type hostsPageData struct {
	Subnet        subnetRow
	Hosts         []hostView
	ICMPAvailable bool
	ScanQueued    bool
}

type eventRow struct {
	TS      int64
	Event   string
	Details string
}

type hostDetailData struct {
	Host   hostView
	Subnet subnetRow
	Events []eventRow
}

func (s *Server) viewSubnet(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var sub subnetRow
	var enabled int
	err = s.db.QueryRow(
		`SELECT id, cidr, name, scan_enabled, scan_interval_sec, COALESCE(last_scan_at, 0), last_scan_err
		 FROM subnets WHERE id = ?`, id,
	).Scan(&sub.ID, &sub.CIDR, &sub.Name, &enabled, &sub.ScanIntervalSec, &sub.LastScanAt, &sub.LastScanErr)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	sub.ScanEnabled = enabled != 0

	rows, err := s.db.Query(
		`SELECT id, subnet_id, ip, mac, hostname_dns, hostname_override, status,
		        COALESCE(first_seen, 0), COALESCE(last_seen, 0), comments, tags
		 FROM hosts WHERE subnet_id = ?`, id,
	)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var hosts []hostView
	for rows.Next() {
		var h hostView
		if err := rows.Scan(&h.ID, &h.SubnetID, &h.IP, &h.MAC, &h.HostnameDNS, &h.HostnameOverride, &h.Status, &h.FirstSeen, &h.LastSeen, &h.Comments, &h.Tags); err != nil {
			http.Error(w, "scan error", http.StatusInternalServerError)
			return
		}
		hosts = append(hosts, h)
	}
	sort.Slice(hosts, func(i, j int) bool {
		ai, _ := netip.ParseAddr(hosts[i].IP)
		bj, _ := netip.ParseAddr(hosts[j].IP)
		return ai.Less(bj)
	})

	flash := ""
	if sub.LastScanErr != "" {
		flash = "last scan error: " + sub.LastScanErr
	}

	s.render(w, r, "hosts", pageData{
		Title: "Hosts — " + sub.CIDR,
		Flash: flash,
		Data: hostsPageData{
			Subnet:        sub,
			Hosts:         hosts,
			ICMPAvailable: s.scanner.ICMPAvailable(),
			ScanQueued:    r.URL.Query().Get("scan") == "queued",
		},
	})
}

func (s *Server) updateHost(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	override := truncate(r.FormValue("hostname_override"), maxHostnameLen)
	comments := truncate(r.FormValue("comments"), maxCommentsLen)
	tags := truncate(normalizeTags(r.FormValue("tags")), maxTagsLen)

	var subnetID int64
	if err := s.db.QueryRow(`SELECT subnet_id FROM hosts WHERE id = ?`, id).Scan(&subnetID); err != nil {
		http.NotFound(w, r)
		return
	}
	if _, err := s.db.Exec(
		`UPDATE hosts SET hostname_override = ?, comments = ?, tags = ? WHERE id = ?`,
		override, comments, tags, id,
	); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, safeRedirect(r.FormValue("redirect"), fmt.Sprintf("/subnets/%d", subnetID)), http.StatusSeeOther)
}

func (s *Server) viewHost(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var h hostView
	err = s.db.QueryRow(
		`SELECT id, subnet_id, ip, mac, hostname_dns, hostname_override, status,
		        COALESCE(first_seen, 0), COALESCE(last_seen, 0), comments, tags
		 FROM hosts WHERE id = ?`, id,
	).Scan(&h.ID, &h.SubnetID, &h.IP, &h.MAC, &h.HostnameDNS, &h.HostnameOverride, &h.Status, &h.FirstSeen, &h.LastSeen, &h.Comments, &h.Tags)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	var sub subnetRow
	var enabled int
	err = s.db.QueryRow(
		`SELECT id, cidr, name, scan_enabled, scan_interval_sec, COALESCE(last_scan_at, 0), last_scan_err
		 FROM subnets WHERE id = ?`, h.SubnetID,
	).Scan(&sub.ID, &sub.CIDR, &sub.Name, &enabled, &sub.ScanIntervalSec, &sub.LastScanAt, &sub.LastScanErr)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	sub.ScanEnabled = enabled != 0

	rows, err := s.db.Query(
		`SELECT ts, event, details FROM scan_events WHERE host_id = ? ORDER BY id DESC LIMIT 100`, id,
	)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var events []eventRow
	for rows.Next() {
		var e eventRow
		if err := rows.Scan(&e.TS, &e.Event, &e.Details); err != nil {
			http.Error(w, "scan error", http.StatusInternalServerError)
			return
		}
		events = append(events, e)
	}

	title := h.IP
	if h.HostnameOverride != "" {
		title = h.HostnameOverride + " (" + h.IP + ")"
	} else if h.HostnameDNS != "" {
		title = h.HostnameDNS + " (" + h.IP + ")"
	}

	s.render(w, r, "host", pageData{
		Title: title,
		Data:  hostDetailData{Host: h, Subnet: sub, Events: events},
	})
}

// ---- events feed ----

type eventFeedRow struct {
	EventID           int64
	HostID            int64
	SubnetID          int64
	SubnetCIDR        string
	IP                string
	EffectiveHostname string
	TS                int64
	Event             string
	Details           string
}

type eventsPageData struct {
	Events     []eventFeedRow
	HasMore    bool
	NextCursor int64
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	const limit = 100
	var beforeID int64
	if v := r.URL.Query().Get("before"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			beforeID = n
		}
	}

	query := `
		SELECT e.id, h.id, h.subnet_id, sn.cidr, h.ip,
		       h.hostname_override, h.hostname_dns, e.ts, e.event, e.details
		FROM scan_events e
		JOIN hosts h   ON e.host_id   = h.id
		JOIN subnets sn ON h.subnet_id = sn.id`
	var rows *sql.Rows
	var err error
	if beforeID > 0 {
		rows, err = s.db.Query(query+` WHERE e.id < ? ORDER BY e.id DESC LIMIT ?`, beforeID, limit+1)
	} else {
		rows, err = s.db.Query(query+` ORDER BY e.id DESC LIMIT ?`, limit+1)
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var events []eventFeedRow
	for rows.Next() {
		var e eventFeedRow
		var override, dns string
		if err := rows.Scan(&e.EventID, &e.HostID, &e.SubnetID, &e.SubnetCIDR, &e.IP,
			&override, &dns, &e.TS, &e.Event, &e.Details); err != nil {
			http.Error(w, "scan error", http.StatusInternalServerError)
			return
		}
		if override != "" {
			e.EffectiveHostname = override
		} else {
			e.EffectiveHostname = dns
		}
		events = append(events, e)
	}

	hasMore := len(events) > limit
	var nextCursor int64
	if hasMore {
		events = events[:limit]
		nextCursor = events[len(events)-1].EventID
	}

	s.render(w, r, "events", pageData{
		Title: "Recent activity",
		Data:  eventsPageData{Events: events, HasMore: hasMore, NextCursor: nextCursor},
	})
}

// ---- CSV export ----

func (s *Server) exportSubnet(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var cidr string
	if err := s.db.QueryRow(`SELECT cidr FROM subnets WHERE id = ?`, id).Scan(&cidr); err != nil {
		http.NotFound(w, r)
		return
	}
	rows, err := s.db.Query(
		`SELECT ip, hostname_dns, hostname_override, mac, status,
		        COALESCE(first_seen, 0), COALESCE(last_seen, 0), comments, tags
		 FROM hosts WHERE subnet_id = ?`, id,
	)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type exportRow struct {
		IP, DNS, Override, MAC, Status string
		FirstSeen, LastSeen            int64
		Comments, Tags                 string
	}
	var hosts []exportRow
	for rows.Next() {
		var h exportRow
		if err := rows.Scan(&h.IP, &h.DNS, &h.Override, &h.MAC, &h.Status,
			&h.FirstSeen, &h.LastSeen, &h.Comments, &h.Tags); err != nil {
			http.Error(w, "scan error", http.StatusInternalServerError)
			return
		}
		hosts = append(hosts, h)
	}
	sort.Slice(hosts, func(i, j int) bool {
		a, _ := netip.ParseAddr(hosts[i].IP)
		b, _ := netip.ParseAddr(hosts[j].IP)
		return a.Less(b)
	})

	safeName := strings.NewReplacer("/", "-", " ", "_", ":", "-").Replace(cidr)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="subnet_%s.csv"`, safeName))

	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"ip", "hostname", "hostname_dns", "hostname_override", "mac", "status", "first_seen", "last_seen", "comments", "tags"})
	for _, h := range hosts {
		effective := h.Override
		if effective == "" {
			effective = h.DNS
		}
		first, last := "", ""
		if h.FirstSeen > 0 {
			first = time.Unix(h.FirstSeen, 0).UTC().Format(time.RFC3339)
		}
		if h.LastSeen > 0 {
			last = time.Unix(h.LastSeen, 0).UTC().Format(time.RFC3339)
		}
		_ = cw.Write([]string{h.IP, effective, h.DNS, h.Override, h.MAC, h.Status, first, last, h.Comments, h.Tags})
	}
	cw.Flush()
}

func (s *Server) exportEvents(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`
		SELECT sn.cidr, h.ip, h.hostname_override, h.hostname_dns,
		       e.ts, e.event, e.details
		FROM scan_events e
		JOIN hosts h   ON e.host_id   = h.id
		JOIN subnets sn ON h.subnet_id = sn.id
		ORDER BY e.id DESC
	`)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="events.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"timestamp", "subnet", "ip", "hostname", "event", "details"})
	for rows.Next() {
		var cidr, ip, override, dns, event, details string
		var ts int64
		if err := rows.Scan(&cidr, &ip, &override, &dns, &ts, &event, &details); err != nil {
			return
		}
		hn := override
		if hn == "" {
			hn = dns
		}
		_ = cw.Write([]string{
			time.Unix(ts, 0).UTC().Format(time.RFC3339),
			cidr, ip, hn, event, details,
		})
	}
	cw.Flush()
}

func normalizeTags(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// ---- settings ----

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	notice := ""
	if r.URL.Query().Get("saved") == "1" {
		notice = "Settings saved."
	}
	s.render(w, r, "settings", pageData{Title: "Settings", Notice: notice})
}

func (s *Server) postSettings(w http.ResponseWriter, r *http.Request) {
	uid, ok := r.Context().Value(userIDKey).(int64)
	if !ok || uid == 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	theme := "light"
	if r.FormValue("theme") == "dark" {
		theme = "dark"
	}
	layout := "normal"
	if r.FormValue("full_width") != "" {
		layout = "wide"
	}
	if _, err := s.db.Exec(
		`UPDATE users SET ui_theme = ?, ui_layout = ? WHERE id = ?`,
		theme, layout, uid,
	); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	// Remove any stale pre-DB prefs cookie so the browser stops carrying them.
	for _, n := range []string{"theme", "layout"} {
		http.SetCookie(w, &http.Cookie{Name: n, Value: "", Path: "/", MaxAge: -1})
	}
	http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
}

// ---- bulk operations ----

func (s *Server) bulkHosts(w http.ResponseWriter, r *http.Request) {
	subnetID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	action := r.FormValue("action")
	rawIDs := r.Form["host_ids"]
	var ids []int64
	for _, v := range rawIDs {
		if n, perr := strconv.ParseInt(v, 10, 64); perr == nil {
			ids = append(ids, n)
		}
	}
	dest := fmt.Sprintf("/subnets/%d", subnetID)
	if len(ids) == 0 {
		http.Redirect(w, r, dest, http.StatusSeeOther)
		return
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	idArgs := make([]any, len(ids))
	for i, id := range ids {
		idArgs[i] = id
	}

	var query string
	var args []any
	switch action {
	case "delete":
		query = fmt.Sprintf(`DELETE FROM hosts WHERE subnet_id = ? AND id IN (%s)`, placeholders)
		args = append([]any{subnetID}, idArgs...)
	case "set_comment":
		comments := truncate(r.FormValue("comments"), maxCommentsLen)
		query = fmt.Sprintf(`UPDATE hosts SET comments = ? WHERE subnet_id = ? AND id IN (%s)`, placeholders)
		args = append([]any{comments, subnetID}, idArgs...)
	case "set_tags":
		tags := truncate(normalizeTags(r.FormValue("tags")), maxTagsLen)
		query = fmt.Sprintf(`UPDATE hosts SET tags = ? WHERE subnet_id = ? AND id IN (%s)`, placeholders)
		args = append([]any{tags, subnetID}, idArgs...)
	default:
		http.Redirect(w, r, dest, http.StatusSeeOther)
		return
	}
	if _, err := s.db.Exec(query, args...); err != nil {
		s.logger.Error("bulk", "action", action, "err", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, dest, http.StatusSeeOther)
}
