package main

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	LoadEnv()
	m := NewSFTPmanager(8)
	b := NewBlazer()
	go m.Run()

	mux := http.NewServeMux()
	mux.Handle("/test-sftp", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleTestSFTP(m, w, r)
	}))
	mux.Handle("/test-blazer", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleTestBlazer(b, w, r)
	}))

	address := "0.0.0.0:9000"
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

func handleTestSFTP(m *SFTPmanager, w http.ResponseWriter, r *http.Request) {
	start := time.Now()
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
	log.Println(time.Since(start))

}

func handleTestBlazer(b *Blazer, w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var buf bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := b.DownloadStreamFile(ctx, 0, "wizzard.png", &buf); err != nil {
		log.Println(err)
		http.Error(w, "Error downloading file", http.StatusInternalServerError)
		return
	}

	if _, err := buf.WriteTo(w); err != nil {
		log.Println("Error writing response:", err)
	}
	log.Println(time.Since(start))

}
