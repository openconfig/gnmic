package utils

import (
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/gorilla/mux"
)

// CanonicalPathRedirect returns a handler, meant to be the router's
// NotFoundHandler, that redirects to the canonical form of the escaped
// request path when that form matches one of the router's routes.
//
// It replaces the redirects done by mux's path cleaning and StrictSlash,
// which rebuild the Location from a modified URL.Path and therefore drop or
// double-encode escapes such as "%2F". The router must be configured with
// SkipClean(true) and without StrictSlash for this handler to be reached.
//
// The canonical form is the cleaned path; with strictSlash, the cleaned path
// with its trailing slash removed or added is tried as well. Requests with
// no matching canonical form get a 404.
func CanonicalPathRedirect(router *mux.Router, strictSlash bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escaped := r.URL.EscapedPath()
		cleaned := cleanPath(escaped)
		candidates := []string{cleaned}
		if strictSlash && cleaned != "/" {
			if strings.HasSuffix(cleaned, "/") {
				candidates = append(candidates, strings.TrimSuffix(cleaned, "/"))
			} else {
				candidates = append(candidates, cleaned+"/")
			}
		}
		for _, p := range candidates {
			if p == escaped || !isLocalPath(p) {
				continue
			}
			decoded, err := url.PathUnescape(p)
			if err != nil {
				continue
			}
			// scheme and host are left out: an absolute-form request
			// target would otherwise redirect to a client-supplied host.
			u := url.URL{Path: decoded, RawPath: p, RawQuery: r.URL.RawQuery}
			probe := r.Clone(r.Context())
			probe.URL = &u
			var match mux.RouteMatch
			if router.Match(probe, &match) && match.MatchErr == nil {
				w.Header().Set("Location", u.String())
				w.WriteHeader(http.StatusMovedPermanently)
				return
			}
		}
		http.NotFound(w, r)
	})
}

// isLocalPath reports whether p is a path on the same host: browsers treat
// a leading "//" or "/\" as a scheme-relative URL to another host.
func isLocalPath(p string) bool {
	return len(p) > 0 && p[0] == '/' && (len(p) == 1 || (p[1] != '/' && p[1] != '\\'))
}

// cleanPath mirrors mux's path cleaning: it returns the canonical form of p,
// keeping a trailing slash.
func cleanPath(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	np := path.Clean(p)
	if p[len(p)-1] == '/' && np != "/" {
		np += "/"
	}
	return np
}
