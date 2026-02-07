package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (s *Server) createSession(w http.ResponseWriter, userID primitive.ObjectID) error {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return err
	}

	raw := base64.RawURLEncoding.EncodeToString(b)
	th := sha256.Sum256([]byte(raw))

	now := time.Now().UTC()
	exp := now.Add(sessionTTL)

	doc := sessionDoc{
		UserID:    userID,
		TokenHash: th[:],
		ExpiresAt: exp,
		CreatedAt: now,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.sessions.InsertOne(ctx, doc)
	if err != nil {
		return err
	}

	setCookie(w, raw, exp, s.devMode)
	return nil
}

func setCookie(w http.ResponseWriter, value string, exp time.Time, devMode bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     "/",
		Expires:  exp,
		MaxAge:   int(time.Until(exp).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   !devMode,
	})
}

func clearCookie(w http.ResponseWriter, devMode bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   !devMode,
	})
}

func readSessionCookie(r *http.Request) (string, error) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return "", err
	}
	if c.Value == "" {
		return "", errors.New("empty cookie")
	}
	return c.Value, nil
}
