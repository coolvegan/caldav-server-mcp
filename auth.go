package main

import (
	"net/http"
)

func basicAuth(user, pass string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, p, ok := r.BasicAuth()
			if !ok || u != user || p != pass {
				w.Header().Set("WWW-Authenticate", `Basic realm="CalDAV"`)
				http.Error(w, "Unauthorized", 401)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
