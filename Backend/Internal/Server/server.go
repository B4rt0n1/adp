package server

import (
	"regexp"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/mongo"
)

type Server struct {
	client           *mongo.Client
	db               *mongo.Database
	users            *mongo.Collection
	sessions         *mongo.Collection
	code_submissions *mongo.Collection
	tasks            *mongo.Collection
	staticDir        string
	devMode          bool

	rateMu   sync.Mutex
	rateByIP map[string][]time.Time

	emailRegex *regexp.Regexp
	router     *chi.Mux
}
