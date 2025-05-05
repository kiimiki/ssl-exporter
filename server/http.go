package server

import (
	"log"
	"net/http"
)

func Start() {
	http.Handle("/", http.FileServer(http.Dir(".")))
	log.Println("Starting HTTP server on :9115")
	log.Fatal(http.ListenAndServe(":9115", nil))
}
