package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"golang.org/x/crypto/bcrypt"
)

const (
	cookieName       = "goedu_session"
	sessionTTL       = 7 * 24 * time.Hour
	loginRateWindow  = 1 * time.Minute
	loginRateMaxHits = 10
	bcryptCost       = 12
)

type server struct {
	client    *mongo.Client
	db        *mongo.Database
	users     *mongo.Collection
	sessions  *mongo.Collection
	staticDir string
	devMode   bool

	rateMu   sync.Mutex
	rateByIP map[string][]time.Time

	emailRegex *regexp.Regexp
}

type userDoc struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Name      string             `bson:"name"`
	Email     string             `bson:"email"`
	PassHash  []byte             `bson:"passHash"`
	CreatedAt time.Time          `bson:"createdAt"`
}

type sessionDoc struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    primitive.ObjectID `bson:"userId"`
	TokenHash []byte             `bson:"tokenHash"`
	ExpiresAt time.Time          `bson:"expiresAt"`
	CreatedAt time.Time          `bson:"createdAt"`
}

type registerReq struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type meResp struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func main() {
	_ = godotenv.Load()

	log.Println("PORT =", os.Getenv("PORT"))
	log.Println("DEV =", os.Getenv("DEV"))

	port := getenv("PORT", "8080")
	staticDir := getenv("STATIC_DIR", ".")
	devMode := os.Getenv("DEV") == "1"

	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		log.Fatal("MONGODB_URI missing (use your Atlas mongodb+srv://... connection string)")
	}
	dbName := getenv("MONGODB_DB", "goedu")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal(err)
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		log.Fatal(err)
	}

	db := client.Database(dbName)
	s := &server{
		client:     client,
		db:         db,
		users:      db.Collection("users"),
		sessions:   db.Collection("sessions"),
		staticDir:  staticDir,
		devMode:    devMode,
		rateByIP:   make(map[string][]time.Time),
		emailRegex: regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`),
	}

	if err := s.ensureIndexes(context.Background()); err != nil {
		log.Fatal(err)
	}

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

	addr := ":" + port
	log.Printf("Server: http://localhost%s (MongoDB Atlas DB=%s)", addr, dbName)
	log.Fatal(http.ListenAndServe(addr, r))
}

func (s *server) ensureIndexes(ctx context.Context) error {
	_, err := s.users.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}

	_, err = s.sessions.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "tokenHash", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}

	ttlSeconds := int32(0)
	_, err = s.sessions.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "expiresAt", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(ttlSeconds),
	})
	return err
}

func (s *server) withSecurity(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; base-uri 'self'; frame-ancestors 'none'")

		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch || r.Method == http.MethodDelete {
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

func (s *server) allowRequest(r *http.Request) bool {
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

func (s *server) handleRegister(w http.ResponseWriter, r *http.Request) {
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

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
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

func (s *server) handleLogout(w http.ResponseWriter, r *http.Request) {
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

func (s *server) handleMe(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(ctxUserKey{}).(userDoc)
	writeJSON(w, http.StatusOK, meResp{
		ID:    u.ID.Hex(),
		Name:  u.Name,
		Email: u.Email,
	})
}

type ctxUserKey struct{}

func (s *server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
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

func (s *server) authenticate(r *http.Request) (userDoc, error) {
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

func (s *server) createSession(w http.ResponseWriter, userID primitive.ObjectID) error {
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

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func getenv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

// Static
func (s *server) serveFile(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Join(s.staticDir, filepath.Clean(name))
		http.ServeFile(w, r, p)
	}
}
func (s *server) serveAnyStatic() http.HandlerFunc {
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
