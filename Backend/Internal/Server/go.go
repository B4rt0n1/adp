package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
