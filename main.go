package main

import (
	"log"
	"net/http"

	"github.com/joho/godotenv"
	"github.com/pecet3/concurrent_sftp/multi_sftp"
)

type app struct {
	m *multi_sftp.MultiSFTP
}

func main() {
	LoadEnv()
	c := multi_sftp.Config{
		MaxCacheSize: 1024 * 1024 * 10, // 10 gb
	}
	m := multi_sftp.New(c)
	app := app{
		m: m,
	}
	m.Manager.Run()

	mux := http.NewServeMux()
	mux.HandleFunc("/files/{path}", app.handleServeFiles)
	mux.HandleFunc("/files", app.handleUpload)
	mux.HandleFunc("/", app.handleUploadView)

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
