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
	Role      string             `bson:"role"`
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

type AdminUserResp struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

type codeSubmissionDoc struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    primitive.ObjectID `bson:"userId"`
	LessonID  primitive.ObjectID `bson:"lessonId"`
	Code      string             `bson:"code"`
	CreatedAt time.Time          `bson:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt"`
	IsPassed  bool               `bson:"isPassed"`
}

type RunResp struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

type Task struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Title          string             `bson:"title" json:"title"`
	Tag            string             `bson:"tag" json:"tag"`
	Description    string             `bson:"description" json:"description"`
	StarterCode    string             `bson:"starterCode" json:"starterCode"`
	Order          int                `bson:"order" json:"order"`
	CreatedAt      time.Time          `bson:"createdAt" json:"createdAt"`
	ExpectedOutput string             `bson:"expectedOutput" json:"expectedOutput"`
}

type TaskProgressResp struct {
	Task
	IsCompleted bool `json:"isCompleted"`
	IsLocked    bool `json:"isLocked"`
}
