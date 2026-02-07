package server

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type updateProfileReq struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	u, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req updateProfileReq
	if err := decodeJSON(r, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if req.Name == "" || len(req.Name) > 80 {
		http.Error(w, "Invalid name", http.StatusBadRequest)
		return
	}
	if !s.emailRegex.MatchString(req.Email) || len(req.Email) > 254 {
		http.Error(w, "Invalid email", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	update := bson.M{
		"$set": bson.M{
			"name":  req.Name,
			"email": req.Email,
		},
	}

	_, err = s.users.UpdateByID(ctx, u.ID, update)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			http.Error(w, `{"error":"Email already exists"}`, http.StatusConflict)
			return
		}
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleUploadPhoto(w http.ResponseWriter, r *http.Request) {
	u, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)

	file, header, err := r.FormFile("photo")
	if err != nil {
		http.Error(w, "Invalid file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		http.Error(w, "Only JPG/PNG allowed", http.StatusBadRequest)
		return
	}

	frontendDir := "../FrontEnd"
	profileImagesDir := filepath.Join(frontendDir, "Profile-Images")
	if _, err := os.Stat(profileImagesDir); os.IsNotExist(err) {
		os.MkdirAll(profileImagesDir, os.ModePerm)
	}

	filename := u.ID.Hex() + ext
	path := filepath.Join(profileImagesDir, filename)

	out, err := os.Create(path)
	if err != nil {
		http.Error(w, "Cannot save file", http.StatusInternalServerError)
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		http.Error(w, "Cannot save file", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	_, err = s.users.UpdateByID(ctx, u.ID, map[string]interface{}{
		"$set": map[string]interface{}{"photo": filename},
	})
	if err != nil {
		http.Error(w, "Cannot update user", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"url": "/Profile-Images/" + filename,
	})
}
