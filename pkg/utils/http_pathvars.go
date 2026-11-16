package utils

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
)

// DecodePathVarsMiddleware percent-decodes, exactly once, every variable
// captured by the matched route before the handler reads it via mux.Vars.
//
// It is meant for routers configured with UseEncodedPath, which match on the
// escaped path so that a name such as "rack/router" sent as "rack%2Frouter"
// stays within a single {id} segment, but leave the captured variables
// escaped. A variable that is not a valid escape sequence gets a
// `400 Bad Request` JSON response.
func DecodePathVarsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		if len(vars) == 0 {
			next.ServeHTTP(w, r)
			return
		}
		decoded := make(map[string]string, len(vars))
		for k, v := range vars {
			dv, err := url.PathUnescape(v)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(struct {
					Errors []string `json:"errors"`
				}{Errors: []string{"invalid path parameter " + k + ": " + err.Error()}})
				return
			}
			decoded[k] = dv
		}
		next.ServeHTTP(w, mux.SetURLVars(r, decoded))
	})
}
