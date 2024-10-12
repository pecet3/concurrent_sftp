package main

import (
	"bytes"
	"log"
	"net/http"
	"time"

	"golang.org/x/exp/rand"
)

func (a app) handleServeFiles(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")

	log.Printf("Accessing: %s", path)

	w.Header().Add("Content-Type", "image/png")
	rand.Seed(uint64(time.Now().UnixNano()))
	id := int(rand.Int())
	log.Println(id)
	start := time.Now()
	var buf bytes.Buffer
	f := &File{
		Id:   id,
		Path: path,
	}
	a.m.Download(f, &buf)
	log.Println("SFTP ", time.Since(start))
	if _, err := buf.WriteTo(w); err != nil {
		log.Println("Error writing response:", err)
	}

}
