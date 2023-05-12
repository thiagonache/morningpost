package main

import (
	"log"
	"net/http"

	"github.com/thiagonache/morningpost"
)

func main() {
	server := morningpost.New(&morningpost.MemoryStore{})
	log.Println("Listening http://127.0.0.1:5000")
	log.Fatal(http.ListenAndServe("127.0.0.1:5000", server))
}
