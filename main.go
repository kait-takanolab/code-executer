package main

import (
	"log"
	"net/http"
)

func main() {
	s, err := newServer()
	if err != nil {
		log.Fatal(err)
	}
	log.Fatalf("Error listening on :%v: %v", "8080", http.ListenAndServe(":8080", s))
}
