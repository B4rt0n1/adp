package server

import (
	"context"
	"crypto/sha256"
	"errors"
	"net"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

type ctxUserKey struct{}

type ctxKeyUserID struct{}

func (s *Server) withSecurity(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set(
			"Content-Security-Policy",
			"default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; base-uri 'self'; frame-ancestors 'none'",
		)

		if r.Method == http.MethodPost ||
			r.Method == http.MethodPut ||
			r.Method == http.MethodPatch ||
			r.Method == http.MethodDelete {
			if !isSameOrigin(r) {
				http.Error(w, "Blocked: bad origin", http.StatusForbidden)
				return
			}
		}

		if r.URL.Path == "/api/login" || r.URL.Path == "/api/registration" {
			if !s.allowRequest(r) {
				http.Error(w, "Too many requests, try again later", http.StatusTooManyRequests)
				return
			}
		}

		next(w, r)
	}
}

func isSameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	host := r.Host
	return origin == "http://"+host || origin == "https://"+host
}

func (s *Server) allowRequest(r *http.Request) bool {
	ip := clientIP(r)
	now := time.Now()

	s.rateMu.Lock()
	defer s.rateMu.Unlock()

	hits := s.rateByIP[ip]
	keep := hits[:0]
	cutoff := now.Add(-loginRateWindow)

	for _, t := range hits {
		if t.After(cutoff) {
			keep = append(keep, t)
		}
	}

	if len(keep) >= loginRateMaxHits {
		s.rateByIP[ip] = keep
		return false
	}

	keep = append(keep, now)
	s.rateByIP[ip] = keep
	return true
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := s.authenticate(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), ctxUserKey{}, u)
		next(w, r.WithContext(ctx))
	}
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := r.Context().Value(ctxUserKey{}).(userDoc)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if u.Role != "admin" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}

func (s *Server) authenticate(r *http.Request) (userDoc, error) {
	raw, err := readSessionCookie(r)
	if err != nil {
		return userDoc{}, err
	}

	th := sha256.Sum256([]byte(raw))

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var sess sessionDoc
	if err := s.sessions.FindOne(ctx, bson.M{"tokenHash": th[:]}).Decode(&sess); err != nil {
		return userDoc{}, errors.New("invalid session")
	}

	if time.Now().UTC().After(sess.ExpiresAt) {
		return userDoc{}, errors.New("expired")
	}

	var u userDoc
	if err := s.users.FindOne(ctx, bson.M{"_id": sess.UserID}).Decode(&u); err != nil {
		return userDoc{}, errors.New("user not found")
	}

	return u, nil
}

func UserFromContext(ctx context.Context) (userDoc, bool) {
	u, ok := ctx.Value(ctxUserKey{}).(userDoc)
	return u, ok
}
