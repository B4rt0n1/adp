package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (s *Server) serveFile(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Join(s.staticDir, filepath.Clean(name))
		http.ServeFile(w, r, p)
	}
}

func (s *Server) serveAnyStatic() http.HandlerFunc {
	fs := http.FileServer(http.Dir(s.staticDir))
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "..") {
			http.NotFound(w, r)
			return
		}

		p := filepath.Join(s.staticDir, filepath.Clean(r.URL.Path))
		if _, err := os.Stat(p); err != nil {
			http.NotFound(w, r)
			return
		}

		fs.ServeHTTP(w, r)
	}
}
