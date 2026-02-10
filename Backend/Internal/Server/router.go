package server

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

func (s *Server) setupRouter() {
	r := chi.NewRouter()

	r.Route("/api", func(api chi.Router) {
		api.Post("/registration", s.withSecurity(s.handleRegister))
		api.Post("/login", s.withSecurity(s.handleLogin))
		api.Post("/logout", s.withSecurity(s.requireAuth(s.handleLogout)))
		api.Get("/me", s.withSecurity(s.requireAuth(s.handleMe)))
		api.Post("/update-profile", s.withSecurity(s.requireAuth(s.handleUpdateProfile)))
		api.Post("/upload-photo", s.withSecurity(s.requireAuth(s.handleUploadPhoto)))
		api.Post("/save-code", s.withSecurity(s.requireAuth(s.handleSaveCode)))
	})

	r.Get("/Profile-Images/*", s.serveAnyStatic())
	r.Get("/", s.serveFile("index.html"))
	r.Get("/login", s.serveFile("login.html"))
	r.Get("/handbooks", s.serveFile("handbooks.html"))
	r.Get("/registration", s.serveFile("registration.html"))
	r.Get("/go", s.serveFile("go.html"))
	r.Get("/about", s.serveFile("about.html"))
	r.Get("/profile", s.serveFile("profile.html"))
	r.Get("/*", s.serveAnyStatic())

	s.router = r
	frontendDir := "../FrontEnd"
	profileImagesDir := filepath.Join(frontendDir, "Profile-Images")

	s.router.Get("/Profile-Images/*", func(w http.ResponseWriter, r *http.Request) {
		file := chi.URLParam(r, "*")
		path := filepath.Join(profileImagesDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, path)
	})

}

func (s *Server) Run() error {
	port := getenv("PORT", "8080")
	log.Println("Server: http://localhost:" + port)
	return http.ListenAndServe(":"+port, s.router)
}
