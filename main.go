package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	LoadEnv()
	m := NewSFTPmanager(8)
	go m.Run()

	mux := http.NewServeMux()
	mux.Handle("/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleTest(m, w, r)
	}))

	address := "localhost:9000"
	server := &http.Server{
		Addr:    address,
		Handler: mux,
	}
	log.Printf("Server is listening on: [%s]", address)
	log.Fatal(server.ListenAndServe())
}

func LoadEnv() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
	log.Println("Loaded .env")
}

func handleTest(m *SFTPmanager, w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "image/png")
	_, done := context.WithTimeout(r.Context(), 3*time.Second)
	defer done()
	log.Println(1)
	t := &Task{
		ID:     len(m.tasksMap),
		DoneCh: make(chan bool),
		Writer: w,
	}
	log.Println("new t", t)
	m.waitCh <- t
	isDone, ok := <-t.DoneCh
	if !ok {
		log.Println("not ok")
		return
	}
	if isDone {
		log.Println("done")
	}
}
