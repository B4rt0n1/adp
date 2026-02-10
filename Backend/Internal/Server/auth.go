package server

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerReq
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
	if len(req.Password) < 6 {
		http.Error(w, "Password must be at least 6 characters", http.StatusBadRequest)
		return
	}

	passHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	u := userDoc{
		Name:      req.Name,
		Email:     req.Email,
		PassHash:  passHash,
		Role:      "user",
		CreatedAt: time.Now().UTC(),
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	res, err := s.users.InsertOne(ctx, u)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			http.Error(w, "Email already registered", http.StatusConflict)
			return
		}
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	userID := res.InsertedID.(primitive.ObjectID)
	if err := s.createSession(w, userID); err != nil {
		http.Error(w, "Created user, but session failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, "Registered and logged in")
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := decodeJSON(r, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if !s.emailRegex.MatchString(req.Email) || req.Password == "" {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var u userDoc
	err := s.users.FindOne(ctx, bson.M{"email": req.Email}).Decode(&u)
	if err != nil {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	if bcrypt.CompareHashAndPassword(u.PassHash, []byte(req.Password)) != nil {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	if err := s.createSession(w, u.ID); err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, "Logged in")
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	raw, err := readSessionCookie(r)
	if err == nil {
		th := sha256.Sum256([]byte(raw))
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		_, _ = s.sessions.DeleteOne(ctx, bson.M{"tokenHash": th[:]})
	}

	clearCookie(w, s.devMode)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, "Logged out")
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(ctxUserKey{}).(userDoc)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	_, _ = s.users.DeleteOne(ctx, bson.M{"_id": u.ID})

	raw, err := readSessionCookie(r)
	if err == nil {
		th := sha256.Sum256([]byte(raw))
		_, _ = s.sessions.DeleteOne(ctx, bson.M{"tokenHash": th[:]})
	}

	clearCookie(w, s.devMode)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, "Account deleted")
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(ctxUserKey{}).(userDoc)
	writeJSON(w, http.StatusOK, meResp{
		ID:    u.ID.Hex(),
		Name:  u.Name,
		Email: u.Email,
		Role:  u.Role,
		Photo: u.Photo,
	})
}
