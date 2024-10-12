package main

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/exp/rand"
)

type app struct {
	m *SFTPmanager
	b *Blazer
}

func main() {
	LoadEnv()
	m := NewSFTPmanager(6)
	b := NewBlazer()
	app := app{
		m: m,
		b: b,
	}
	go m.Run()

	mux := http.NewServeMux()
	mux.HandleFunc("/test-sftp", app.handleTestSFTP)
	mux.HandleFunc("/test-blazer", app.handleTestBlazer)

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

func (a app) handleTestSFTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "image/png")
	rand.Seed(uint64(time.Now().UnixNano()))
	id := int(rand.Int())
	log.Println(id)
	start := time.Now()
	var buf bytes.Buffer
	f := &File{
		Id:   id,
		Path: "",
	}
	a.m.Download(f, w)
	log.Println("SFTP ", time.Since(start))
	if _, err := buf.WriteTo(w); err != nil {
		log.Println("Error writing response:", err)
	}

}

func (a app) handleTestBlazer(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	w.Header().Add("Content-Type", "image/png")

	var buf bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.b.DownloadStreamFile(ctx, 0, "wizzard.png", &buf); err != nil {
		log.Println(err)
		http.Error(w, "Error downloading file", http.StatusInternalServerError)
		return
	}

	if _, err := buf.WriteTo(w); err != nil {
		log.Println("Error writing response:", err)
	}
	log.Println("BLAZER: ", time.Since(start))

}
