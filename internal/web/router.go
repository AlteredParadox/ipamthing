// ipamthing — a lightweight single-binary IPAM.
// Copyright (C) 2026 AlteredParadox
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at your
// option) any later version.
//
// This program is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
// FITNESS FOR A PARTICULAR PURPOSE. See the GNU Affero General Public License
// for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package web

import (
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"ipamthing/internal/auth"
	"ipamthing/internal/scan"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

// ServerDeps bundles the dependencies NewRouter needs so the parameter list
// doesn't keep growing as features are added.
type ServerDeps struct {
	Logger       *slog.Logger
	DB           *sql.DB
	Sessions     *auth.SessionStore
	LoginLimiter *auth.LoginLimiter
	Scanner      *scan.Scanner
	Scheduler    *scan.Scheduler
	SetupToken   string
	TrustProxy   bool

	// Version is the build version stamped by the release workflow ("dev"
	// when unstamped); /source pins its AGPL §13 offer to it.
	Version string
	// License and ThirdPartyLicenses are the embedded legal texts served
	// unauthenticated at /license and /third-party-licenses.
	License            []byte
	ThirdPartyLicenses []byte
}

type Server struct {
	logger       *slog.Logger
	db           *sql.DB
	sessions     *auth.SessionStore
	loginLimiter *auth.LoginLimiter
	scanner      *scan.Scanner
	scheduler    *scan.Scheduler
	tmpls        map[string]*template.Template
	trustProxy   bool
	version      string
	license      []byte
	thirdParty   []byte

	setupMu    sync.Mutex
	setupToken string
}

// ipSortKey turns an address or CIDR into a string whose byte order is its
// numeric order, so the browser can sort an IP column with a plain string
// compare (10.0.0.9 before 10.0.0.10, a /16 before the /24 at the same
// base). Anything unparseable sorts as itself.
//
// The key leads with the address family. As16 alone is not enough: it puts
// IPv4 at ::ffff:a.b.c.d, in the middle of IPv6 space, so ::1 would sort
// before every IPv4 address and ::ffff:10.0.0.1 would collide with 10.0.0.1.
func ipSortKey(s string) string {
	if p, err := netip.ParsePrefix(s); err == nil {
		return fmt.Sprintf("%s/%03d", addrSortKey(p.Addr()), p.Bits())
	}
	if a, err := netip.ParseAddr(s); err == nil {
		return addrSortKey(a)
	}
	return s
}

// addrSortKey is "4" or "6" followed by the address as 32 hex digits, so
// all of IPv4 orders before all of IPv6 (IPv4-mapped IPv6 counts as IPv6).
func addrSortKey(a netip.Addr) string {
	family := "6"
	if a.Is4() {
		family = "4"
	}
	return fmt.Sprintf("%s%x", family, a.As16())
}

var funcMap = template.FuncMap{
	"fields": func(s string) []string {
		return strings.Fields(s)
	},
	"ipKey": ipSortKey,
	"relTime": func(u int64) string {
		if u == 0 {
			return "never"
		}
		d := time.Since(time.Unix(u, 0))
		switch {
		case d < 0:
			return "in the future"
		case d < time.Minute:
			return fmt.Sprintf("%ds ago", int(d.Seconds()))
		case d < time.Hour:
			return fmt.Sprintf("%dm ago", int(d.Minutes()))
		case d < 24*time.Hour:
			return fmt.Sprintf("%dh ago", int(d.Hours()))
		default:
			return fmt.Sprintf("%dd ago", int(d.Hours()/24))
		}
	},
}

// parseTemplates pairs every page template with the shared layout. It
// panics on a parse error: the templates are embedded, so a bad one is a
// build defect, not a runtime condition.
func parseTemplates() map[string]*template.Template {
	pages := []string{"setup", "login", "subnets", "hosts", "host", "events", "dashboard", "settings"}
	tmpls := make(map[string]*template.Template, len(pages))
	for _, p := range pages {
		t := template.New("").Funcs(funcMap)
		t = template.Must(t.ParseFS(
			templatesFS,
			"templates/layout.html",
			"templates/"+p+".html",
		))
		tmpls[p] = t
	}
	return tmpls
}

func NewRouter(deps ServerDeps) http.Handler {
	tmpls := parseTemplates()

	s := &Server{
		logger: deps.Logger, db: deps.DB, sessions: deps.Sessions,
		loginLimiter: deps.LoginLimiter, scanner: deps.Scanner,
		scheduler: deps.Scheduler, tmpls: tmpls,
		setupToken: deps.SetupToken, trustProxy: deps.TrustProxy,
		version: deps.Version, license: deps.License, thirdParty: deps.ThirdPartyLicenses,
	}

	mux := http.NewServeMux()

	static, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))

	// AGPL §13: the license texts and the source offer are served to every
	// network user, so they sit outside auth and the first-run redirect.
	mux.Handle("GET /license", serveLegalText(s.license, "license text not embedded in this build"))
	mux.Handle("GET /third-party-licenses", serveLegalText(s.thirdParty, "third-party notices not embedded in this build"))
	mux.HandleFunc("GET /source", s.handleSource)

	mux.HandleFunc("GET /setup", s.getSetup)
	mux.HandleFunc("POST /setup", s.postSetup)
	mux.HandleFunc("GET /login", s.getLogin)
	mux.HandleFunc("POST /login", s.postLogin)
	mux.HandleFunc("POST /logout", s.postLogout)

	mux.Handle("GET /{$}", s.auth(http.HandlerFunc(s.getIndex)))

	mux.Handle("GET /subnets", s.auth(http.HandlerFunc(s.listSubnets)))
	mux.Handle("POST /subnets", s.auth(http.HandlerFunc(s.createSubnet)))
	mux.Handle("GET /subnets/{id}", s.auth(http.HandlerFunc(s.viewSubnet)))
	mux.Handle("POST /subnets/{id}/edit", s.auth(http.HandlerFunc(s.editSubnet)))
	mux.Handle("POST /subnets/{id}/toggle", s.auth(http.HandlerFunc(s.toggleSubnet)))
	mux.Handle("POST /subnets/{id}/scan", s.auth(http.HandlerFunc(s.scanSubnet)))
	mux.Handle("POST /subnets/{id}/delete", s.auth(http.HandlerFunc(s.deleteSubnet)))
	mux.Handle("GET /subnets/{id}/export.csv", s.auth(http.HandlerFunc(s.exportSubnet)))
	mux.Handle("POST /subnets/{id}/hosts/bulk", s.auth(http.HandlerFunc(s.bulkHosts)))

	mux.Handle("GET /hosts/{id}", s.auth(http.HandlerFunc(s.viewHost)))
	mux.Handle("POST /hosts/{id}", s.auth(http.HandlerFunc(s.updateHost)))

	mux.Handle("GET /events", s.auth(http.HandlerFunc(s.events)))
	mux.Handle("GET /events/export.csv", s.auth(http.HandlerFunc(s.exportEvents)))

	mux.Handle("GET /settings", s.auth(http.HandlerFunc(s.getSettings)))
	mux.Handle("POST /settings", s.auth(http.HandlerFunc(s.postSettings)))

	return s.securityHeaders(s.limitBody(s.csrf(s.firstRun(mux))))
}
