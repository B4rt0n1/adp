package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (s *Server) handleSaveCode(w http.ResponseWriter, r *http.Request) {
	var req saveCodeReq
	if err := decodeJSON(r, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	uid, err := primitive.ObjectIDFromHex(req.UserID)

	_, err = s.code_submissions.UpdateOne(
		ctx,
		bson.M{"userId": uid},
		bson.M{
			"$set": bson.M{
				"code":      req.Code,
				"updatedAt": time.Now().UTC(),
				"lessonId":  req.LessonID,
			},
			"$setOnInsert": bson.M{
				"createdAt": time.Now().UTC(),
			},
		},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		fmt.Println(req.UserID)
		fmt.Printf("handleSaveCode UpdateOne error: %v\n", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, "Code saved")
}

func (s *Server) handleGetSavedCode(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(ctxUserKey{}).(userDoc)
	lessonID := r.URL.Query().Get("lessonId")
	if lessonID == "" {
		http.Error(w, "lessonId required", 400)
		return
	}

	filter := bson.M{"userId": u.ID, "lessonId": lessonID}

	var doc struct {
		Code string `bson:"code" json:"code"`
	}

	err := s.code_submissions.FindOne(
		r.Context(), filter,
	).Decode(&doc)

	if err == mongo.ErrNoDocuments {
		writeJSON(w, http.StatusOK, map[string]string{"code": ""})
		return
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, doc)
}

func runHandler(w http.ResponseWriter, r *http.Request) {
	var req saveCodeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	dir, err := os.MkdirTemp("", "gorun-*")
	if err != nil {
		http.Error(w, "tempdir failed", 500)
		return
	}
	defer os.RemoveAll(dir)

	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(req.Code), 0644); err != nil {
		http.Error(w, "write failed", 500)
		return
	}

	args := []string{
		"run", "--rm",
		"-v", dir + ":/work",
		"-w", "/work",
		"golang:latest",
		"go", "run", "main.go",
	}

	cmd := exec.CommandContext(r.Context(), "docker", args...)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb

	exitCode := 0
	if err := cmd.Run(); err != nil {
		// If docker/go run fails, we still return stderr.
		exitCode = 1
	}

	resp := RunResp{
		Stdout:   outb.String(),
		Stderr:   errb.String(),
		ExitCode: exitCode,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
