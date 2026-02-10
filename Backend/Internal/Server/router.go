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
		api.Delete("/delete-account", s.withSecurity(s.requireAuth(s.handleDeleteAccount)))
		api.Get("/me", s.withSecurity(s.requireAuth(s.handleMe)))
		api.Put("/update-profile", s.withSecurity(s.requireAuth(s.handleUpdateProfile)))
		api.Patch("/upload-photo", s.withSecurity(s.requireAuth(s.handleUploadPhoto)))
		api.Post("/save-code", s.withSecurity(s.requireAuth(s.handleSaveCode)))
		api.Post("/run-code", s.withSecurity(s.requireAuth(runHandler)))
		api.Route("/admin", func(admin chi.Router) {
			admin.Get("/users", s.withSecurity(s.requireAuth(s.requireAdmin(s.handleAdminListUsers))))
			admin.Put("/users/{id}", s.withSecurity(s.requireAuth(s.requireAdmin(s.handleAdminUpdateUser))))
			admin.Delete("/users/{id}", s.withSecurity(s.requireAuth(s.requireAdmin(s.handleAdminDeleteUser))))
		})
	})

	s.router = r
	frontendDir := "../FrontEnd"
	profileImagesDir := filepath.Join(frontendDir, "Profile-Images")

	r.Get("/admin", s.withSecurity(s.requireAuth(s.requireAdmin(s.serveFile("admin.html")))))
	r.Get("/", s.serveFile("index.html"))
	r.Get("/login", s.serveFile("login.html"))
	r.Get("/handbooks", s.serveFile("handbooks.html"))
	r.Get("/registration", s.serveFile("registration.html"))
	r.Get("/go", s.serveFile("go.html"))
	r.Get("/about", s.serveFile("about.html"))
	r.Get("/profile", s.serveFile("profile.html"))

	r.Handle("/Profile-Images/*",
		http.StripPrefix("/Profile-Images/",
			http.FileServer(http.Dir(profileImagesDir)),
		),
	)

	r.Get("/*", s.serveAnyStatic())
	r.Get("/savedCode", s.withSecurity(s.requireAuth(s.handleGetSavedCode)))

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
