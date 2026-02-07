package main

import (
	"log"

	"goedu/internal/server"
)

func main() {
	s, err := server.New()
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(s.Run())
}
