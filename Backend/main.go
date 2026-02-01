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

func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var u User
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		http.Error(w, "Error parsing JSON", http.StatusBadRequest)
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

	w.WriteHeader(http.StatusCreated)
	fmt.Fprint(w, "Registration successful")
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var creds User
	json.NewDecoder(r.Body).Decode(&creds)

	mutex.RLock()
	storedUser, exists := usersMap[creds.Email]
	mutex.RUnlock()

	if !exists || storedUser.Password != creds.Password {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	fmt.Fprint(w, "Login successful")
}

func main() {
	go systemMonitor()

	fs := http.FileServer(http.Dir("./FrontEnd"))
	http.Handle("/", fs)

	http.HandleFunc("/api/register", registerHandler)
	http.HandleFunc("/api/login", loginHandler)

	port := ":8080"
	fmt.Printf("Server starting at http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
