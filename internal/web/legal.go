package web

import (
	"net/http"
	"regexp"
)

// sourceBaseURL is where the Corresponding Source is published. AGPL §13
// requires a network user be offered the source of the version they are
// interacting with — a downstream fork that changes this (and rebuilds) makes
// /source point at ITS source, which is exactly the intent.
const sourceBaseURL = "https://github.com/AlteredParadox/ipamthing"

// pinnable matches a release tag ("v1.2.3") or a bare commit hash, the two
// version strings that name a tree on GitHub. `git describe` output such as
// "v1.2.3-4-gabc1234" or "abc1234-dirty" does not, and must not be pinned.
var pinnable = regexp.MustCompile(`^(v\d+\.\d+\.\d+|[0-9a-f]{7,40})$`)

// Version returns the build version ("dev" for unstamped builds).
func (s *Server) Version() string {
	if s.version == "" {
		return "dev"
	}
	return s.version
}

// sourceURL is the AGPL §13 offer target. A stamped release pins to its tag,
// so the source you are offered is the source that is running; any other
// build can only point at the repository.
func (s *Server) sourceURL() string {
	if v := s.Version(); pinnable.MatchString(v) {
		return sourceBaseURL + "/tree/" + v
	}
	return sourceBaseURL
}

// isLegalPath reports whether a route is the license/source offer. AGPL §13
// obliges the offer to every network user, so these routes bypass both the
// login wall and the first-run redirect to /setup. The match is exact so no
// other route escapes.
func isLegalPath(p string) bool {
	return p == "/license" || p == "/third-party-licenses" || p == "/source"
}

// serveLegalText serves an embedded text blob as plain UTF-8.
func serveLegalText(body []byte, missing string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if len(body) == 0 {
			http.Error(w, missing, http.StatusInternalServerError)
			return
		}
		w.Header().Set(headerContentType, "text/plain; charset=utf-8")
		// Tied to the binary, so it only changes when the build does.
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(body)
	}
}

// handleSource implements the AGPL §13 source offer.
func (s *Server) handleSource(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, s.sourceURL(), http.StatusFound)
}
