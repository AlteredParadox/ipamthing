package web

import (
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
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

type Server struct {
	logger       *slog.Logger
	db           *sql.DB
	sessions     *auth.SessionStore
	loginLimiter *auth.LoginLimiter
	scanner      *scan.Scanner
	scheduler    *scan.Scheduler
	tmpls        map[string]*template.Template
	trustProxy   bool

	setupMu    sync.Mutex
	setupToken string // required for /setup; cleared after first admin is created
}

var funcMap = template.FuncMap{
	"fields": func(s string) []string {
		return strings.Fields(s)
	},
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

func NewRouter(logger *slog.Logger, db *sql.DB, sessions *auth.SessionStore, loginLimiter *auth.LoginLimiter, scanner *scan.Scanner, scheduler *scan.Scheduler, setupToken string, trustProxy bool) http.Handler {
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

	s := &Server{
		logger: logger, db: db, sessions: sessions, loginLimiter: loginLimiter,
		scanner: scanner, scheduler: scheduler, tmpls: tmpls,
		setupToken: setupToken, trustProxy: trustProxy,
	}

	mux := http.NewServeMux()

	static, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))

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
