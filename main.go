package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	LoadEnv()
	m := NewSFTPmanager(8)
	go m.Run()
	log.Println("new t")
	t := &Task{
		DoneCh: make(chan bool),
	}
	log.Println("3", t)
	time.Sleep(time.Second * 1)
	log.Println("2", t)
	time.Sleep(time.Second * 1)
	log.Println("1", t)
	time.Sleep(time.Second * 1)
	m.waitCh <- t
	<-t.DoneCh
	log.Println("done")

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
	log.Println(1)
	start := time.Now()
	t := &Task{
		DoneCh: make(chan bool),
	}
	log.Println("new t", t)
	m.waitCh <- t
	<-t.DoneCh
	elapsed := time.Since(start)
	log.Println("done")
	w.Write([]byte(fmt.Sprintf(`task done in: %v`, elapsed)))
}
