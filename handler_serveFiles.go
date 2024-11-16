package main

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"time"

	"github.com/pecet3/concurrent_sftp/multi_sftp"
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
	f := &multi_sftp.File{
		Path: path,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()
	a.m.Manager.Download(ctx, f, &buf)

	log.Println("SFTP ", time.Since(start))
	if _, err := buf.WriteTo(w); err != nil {
		log.Println("Error writing response:", err)
	}

}
