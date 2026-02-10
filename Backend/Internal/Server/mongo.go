package server

import (
	"context"
	"os"
	"regexp"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func New() (*Server, error) {
	_ = godotenv.Load()

	port := getenv("PORT", "8080")
	_ = port
	staticDir := getenv("STATIC_DIR", ".")
	devMode := os.Getenv("DEV") == "1"

	mongoURI := os.Getenv("MONGODB_URI")
	dbName := getenv("MONGODB_DB", "goedu")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, err
	}

	db := client.Database(dbName)

	s := &Server{
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
		return nil, err
	}

	s.setupRouter()
	return s, nil
}
