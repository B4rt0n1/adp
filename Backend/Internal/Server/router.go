package server

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) setupRouter() {
	r := chi.NewRouter()

	r.Route("/api", func(api chi.Router) {
		api.Post("/registration", s.withSecurity(s.handleRegister))
		api.Post("/login", s.withSecurity(s.handleLogin))
		api.Post("/logout", s.withSecurity(s.requireAuth(s.handleLogout)))
		api.Get("/me", s.withSecurity(s.requireAuth(s.handleMe)))
	})

	r.Get("/", s.serveFile("index.html"))
	r.Get("/login", s.serveFile("login.html"))
	r.Get("/handbooks", s.serveFile("handbooks.html"))
	r.Get("/registration", s.serveFile("registration.html"))
	r.Get("/go", s.serveFile("go.html"))
	r.Get("/about", s.serveFile("about.html"))
	r.Get("/*", s.serveAnyStatic())

	s.router = r
}

func (s *Server) Run() error {
	port := getenv("PORT", "8080")
	log.Println("Server: http://localhost:" + port)
	return http.ListenAndServe(":"+port, s.router)
}
