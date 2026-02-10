package server

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type userDoc struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Name      string             `bson:"name"`
	Email     string             `bson:"email"`
	PassHash  []byte             `bson:"passHash"`
	Photo     string             `bson:"photo,omitempty"`
	CreatedAt time.Time          `bson:"createdAt"`
}

type sessionDoc struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    primitive.ObjectID `bson:"userId"`
	TokenHash []byte             `bson:"tokenHash"`
	ExpiresAt time.Time          `bson:"expiresAt"`
	CreatedAt time.Time          `bson:"createdAt"`
}

type codeSubmissionDoc struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    primitive.ObjectID `bson:"userId"`
	LessonID  primitive.ObjectID `bson:"lessonId"`
	Code      string             `bson:"code"`
	CreatedAt time.Time          `bson:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt"`
}
