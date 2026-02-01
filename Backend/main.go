package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type User struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type PublicUser struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

var (
	usersMap = make(map[string]User)
	mutex    sync.RWMutex
)

func systemMonitor() {
	for {
		time.Sleep(1 * time.Minute)
		mutex.RLock()
		count := len(usersMap)
		mutex.RUnlock()
		log.Printf("[BG-STATS] Total users registered: %d", count)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func toPublic(u User) PublicUser {
	return PublicUser{Name: u.Name, Email: u.Email}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var creds User
	if err := readJSON(r, &creds); err != nil {
		http.Error(w, "Error parsing JSON", http.StatusBadRequest)
		return
	}
	if creds.Email == "" || creds.Password == "" {
		http.Error(w, "Email and password required", http.StatusBadRequest)
		return
	}

	mutex.RLock()
	storedUser, exists := usersMap[creds.Email]
	mutex.RUnlock()

	if !exists || storedUser.Password != creds.Password {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "Login successful",
		"user":    toPublic(storedUser),
	})
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")

	switch r.Method {

	case http.MethodPost:
		var u User
		if err := readJSON(r, &u); err != nil {
			http.Error(w, "Error parsing JSON", http.StatusBadRequest)
			return
		}
		if u.Email == "" || u.Password == "" || u.Name == "" {
			http.Error(w, "Name, email, and password required", http.StatusBadRequest)
			return
		}

		mutex.Lock()
		if _, exists := usersMap[u.Email]; exists {
			mutex.Unlock()
			http.Error(w, "User already exists", http.StatusConflict)
			return
		}
		usersMap[u.Email] = u
		mutex.Unlock()

		writeJSON(w, http.StatusCreated, map[string]any{
			"message": "User created",
			"user":    toPublic(u),
		})

	case http.MethodGet:
		mutex.RLock()
		defer mutex.RUnlock()

		if email != "" {
			u, ok := usersMap[email]
			if !ok {
				http.Error(w, "User not found", http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, toPublic(u))
			return
		}

		out := make([]PublicUser, 0, len(usersMap))
		for _, u := range usersMap {
			out = append(out, toPublic(u))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"count": len(out),
			"users": out,
		})

	case http.MethodPut:
		if email == "" {
			http.Error(w, "Missing required query parameter: email", http.StatusBadRequest)
			return
		}

		var patch User
		if err := readJSON(r, &patch); err != nil {
			http.Error(w, "Error parsing JSON", http.StatusBadRequest)
			return
		}

		mutex.Lock()
		u, ok := usersMap[email]
		if !ok {
			mutex.Unlock()
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		if patch.Name != "" {
			u.Name = patch.Name
		}
		if patch.Password != "" {
			u.Password = patch.Password
		}

		if patch.Email != "" && patch.Email != email {
			if _, exists := usersMap[patch.Email]; exists {
				mutex.Unlock()
				http.Error(w, "New email already in use", http.StatusConflict)
				return
			}
			delete(usersMap, email)
			u.Email = patch.Email
			usersMap[u.Email] = u
		} else {
			usersMap[email] = u
		}
		mutex.Unlock()

		writeJSON(w, http.StatusOK, map[string]any{
			"message": "User updated",
			"user":    toPublic(u),
		})

	case http.MethodDelete:
		if email == "" {
			http.Error(w, "Missing required query parameter: email", http.StatusBadRequest)
			return
		}

		mutex.Lock()
		_, ok := usersMap[email]
		if ok {
			delete(usersMap, email)
		}
		mutex.Unlock()

		if !ok {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"message": "User deleted",
			"email":   email,
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func main() {
	go systemMonitor()

	fs := http.FileServer(http.Dir("../FrontEnd"))
	http.Handle("/", fs)

	http.HandleFunc("/api/users", usersHandler)

	http.HandleFunc("/api/login", loginHandler)

	port := ":8080"
	fmt.Printf("Server starting at http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
