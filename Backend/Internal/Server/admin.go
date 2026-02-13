package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.GetAllUsers(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleAdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Name string `json:"name"`
		Role string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", 400)
		return
	}

	err := s.UpdateUserByAdmin(r.Context(), id, req.Name, req.Role)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := s.DeleteUser(r.Context(), id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) GetAllUsers(ctx context.Context) ([]AdminUserResp, error) {
	cur, err := s.users.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var docs []userDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}

	users := make([]AdminUserResp, 0, len(docs))
	for _, u := range docs {
		users = append(users, AdminUserResp{
			ID:        u.ID.Hex(),
			Name:      u.Name,
			Email:     u.Email,
			Role:      u.Role,
			CreatedAt: u.CreatedAt,
		})
	}

	return users, nil
}

func (s *Server) UpdateUserByAdmin(ctx context.Context, id, name, role string) error {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = s.users.UpdateByID(ctx,
		objID,
		bson.M{"$set": bson.M{"name": name, "role": role}},
	)
	return err
}

func (s *Server) DeleteUser(ctx context.Context, id string) error {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = s.users.DeleteOne(ctx, bson.M{
		"_id": objID,
	})
	return err
}

func (s *Server) handleAdminCreateTask(w http.ResponseWriter, r *http.Request) {
	var task Task
	if err := decodeJSON(r, &task); err != nil {
		http.Error(w, "Invalid JSON", 400)
		return
	}
	task.ID = primitive.NewObjectID()
	task.CreatedAt = time.Now().UTC()

	_, err := s.tasks.InsertOne(r.Context(), task)
	if err != nil {
		http.Error(w, "DB Error", 500)
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func (s *Server) handleAdminDeleteTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := primitive.ObjectIDFromHex(idStr)
	_, err := s.tasks.DeleteOne(r.Context(), bson.M{"_id": id})
	if err != nil {
		http.Error(w, "Delete failed", 500)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminUpdateTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", 400)
		return
	}

	var req Task
	if err := decodeJSON(r, &req); err != nil {
		http.Error(w, "Invalid JSON", 400)
		return
	}

	update := bson.M{
		"$set": bson.M{
			"title":          req.Title,
			"tag":            req.Tag,
			"description":    req.Description,
			"starterCode":    req.StarterCode,
			"order":          req.Order,
			"expectedOutput": req.ExpectedOutput,
		},
	}

	_, err = s.tasks.UpdateOne(r.Context(), bson.M{"_id": id}, update)
	if err != nil {
		http.Error(w, "Update failed", 500)
		return
	}

	w.WriteHeader(http.StatusOK)
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
