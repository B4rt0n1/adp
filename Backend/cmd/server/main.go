package main

import (
	"log"

	server "goedu/Internal/Server"
)

func main() {
	s, err := server.New()
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(s.Run())
}
