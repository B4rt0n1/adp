package server

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
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

func (s *Server) handleRunAndCheck(w http.ResponseWriter, r *http.Request) {
	u, ok := r.Context().Value(ctxUserKey{}).(userDoc)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req saveCodeReq
	if err := decodeJSON(r, &req); err != nil {
		http.Error(w, "Bad JSON", http.StatusBadRequest)
		return
	}

	dir, err := os.MkdirTemp("", "gorun-*")
	if err != nil {
		http.Error(w, "Temp dir failed", 500)
		return
	}
	defer os.RemoveAll(dir)
	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte(req.Code), 0644)

	args := []string{"run", "--rm", "-v", dir + ":/work", "-w", "/work", "golang:alpine", "go", "run", "main.go"}
	cmd := exec.CommandContext(r.Context(), "docker", args...)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	exitCode := 0
	if err := cmd.Run(); err != nil {
		exitCode = 1
	}

	isCorrect := false
	lessonOID, err := primitive.ObjectIDFromHex(req.LessonID)
	if err == nil {
		var task Task
		err = s.tasks.FindOne(r.Context(), bson.M{"_id": lessonOID}).Decode(&task)

		if err == nil && task.ExpectedOutput != "" {
			actual := strings.TrimSpace(outb.String())
			expected := strings.TrimSpace(task.ExpectedOutput)

			if actual == expected && actual != "" {
				isCorrect = true
				s.markTaskAsCompleted(r.Context(), u.ID, lessonOID, req.Code)
			}
		}
	}

	writeJSON(w, http.StatusOK, struct {
		Stdout    string `json:"stdout"`
		Stderr    string `json:"stderr"`
		ExitCode  int    `json:"exitCode"`
		IsCorrect bool   `json:"isCorrect"`
	}{
		Stdout:    outb.String(),
		Stderr:    errb.String(),
		ExitCode:  exitCode,
		IsCorrect: isCorrect,
	})
}

func (s *Server) markTaskAsCompleted(ctx context.Context, userID, lessonID primitive.ObjectID, code string) {
	now := time.Now().UTC()
	filter := bson.M{"userId": userID, "lessonId": lessonID}
	update := bson.M{
		"$set": bson.M{
			"code":      code,
			"updatedAt": now,
			"isPassed":  true,
		},
		"$setOnInsert": bson.M{
			"createdAt": now,
		},
	}
	opts := options.Update().SetUpsert(true)
	_, _ = s.code_submissions.UpdateOne(ctx, filter, update, opts)
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	opts := options.Find().SetSort(bson.M{"createdAt": -1})
	cur, err := s.tasks.Find(ctx, bson.M{}, opts)
	if err != nil {
		http.Error(w, "Database error", 500)
		return
	}
	defer cur.Close(ctx)

	var tasks []Task = []Task{}
	if err := cur.All(ctx, &tasks); err != nil {
		http.Error(w, "Error decoding tasks", 500)
		return
	}

	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) handleGetLearningPath(w http.ResponseWriter, r *http.Request) {
	var userID primitive.ObjectID
	isGuest := true

	u, ok := r.Context().Value(ctxUserKey{}).(userDoc)
	if ok {
		userID = u.ID
		isGuest = false
	}

	opts := options.Find().SetSort(bson.D{{Key: "order", Value: 1}})
	cur, err := s.tasks.Find(r.Context(), bson.M{}, opts)
	if err != nil {
		http.Error(w, "Database error", 500)
		return
	}
	defer cur.Close(r.Context())

	var tasks []Task
	if err := cur.All(r.Context(), &tasks); err != nil {
		http.Error(w, "Decoding error", 500)
		return
	}

	completedMap := make(map[primitive.ObjectID]bool)

	if !isGuest {
		subCur, err := s.code_submissions.Find(r.Context(), bson.M{"userId": userID})
		if err == nil {
			defer subCur.Close(r.Context())
			var submissions []codeSubmissionDoc
			_ = subCur.All(r.Context(), &submissions)

			for _, sub := range submissions {
				if sub.IsPassed {
					completedMap[sub.LessonID] = true
				}
			}
		}
	}

	type TaskResponse struct {
		Task
		IsCompleted bool `json:"isCompleted"`
		IsLocked    bool `json:"isLocked"`
	}

	responseList := make([]TaskResponse, 0, len(tasks))

	branchUnlocked := make(map[string]bool)

	for _, t := range tasks {
		tag := t.Tag
		if tag == "" {
			tag = "General"
		}

		isCompleted := completedMap[t.ID]

		isNextUnlocked, seen := branchUnlocked[tag]
		if !seen {
			isNextUnlocked = true
		}

		isLocked := !isNextUnlocked
		if isCompleted {
			isLocked = false
		}

		if isCompleted {
			branchUnlocked[tag] = true
		} else {
			branchUnlocked[tag] = false
		}

		responseList = append(responseList, TaskResponse{
			Task:        t,
			IsCompleted: isCompleted,
			IsLocked:    isLocked,
		})
	}

	writeJSON(w, http.StatusOK, responseList)
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	objID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", 400)
		return
	}

	var task Task
	err = s.tasks.FindOne(r.Context(), bson.M{"_id": objID}).Decode(&task)
	if err != nil {
		http.Error(w, "Task not found", 404)
		return
	}

	isCompleted := false
	u, ok := r.Context().Value(ctxUserKey{}).(userDoc)
	if ok {
		count, _ := s.code_submissions.CountDocuments(r.Context(), bson.M{
			"userId":   u.ID,
			"lessonId": objID,
			"isPassed": true,
		})
		if count > 0 {
			isCompleted = true
		}
	}

	response := struct {
		Task
		IsCompleted bool `json:"isCompleted"`
	}{
		Task:        task,
		IsCompleted: isCompleted,
	}

	writeJSON(w, http.StatusOK, response)
}
