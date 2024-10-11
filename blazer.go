package main

import (
	"context"
	"io"
	"log"
	"os"
	"time"

	"github.com/Backblaze/blazer/b2"
)

type Blazer struct {
	Client  *b2.Client
	Buckets []*b2.Bucket
}

func NewBlazer() *Blazer {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer func() {
		cancel()
	}()
	id := os.Getenv("B2_APPLICATION_KEY_ID")
	key := os.Getenv("B2_APPLICATION_KEY")

	b2, err := b2.NewClient(ctx, id, key)
	if err != nil {
		log.Println(err)
	}
	buckets, err := b2.ListBuckets(ctx)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("Blazer is running")
	log.Println(buckets)
	return &Blazer{
		Client:  b2,
		Buckets: buckets,
	}
}

func (b *Blazer) CopyStreamFile(ctx context.Context, src *os.File, dst string) error {
	bucket := b.Buckets[0]

	obj := bucket.Object(dst)
	w := obj.NewWriter(ctx)
	if _, err := io.Copy(w, src); err != nil {
		w.Close()
		return err
	}
	log.Println(obj.Name())
	return w.Close()
}
func (b *Blazer) CopyFile(ctx context.Context, src, dst string) error {
	bucket := b.Buckets[0]
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	obj := bucket.Object(dst)
	w := obj.NewWriter(ctx)
	if _, err := io.Copy(w, f); err != nil {
		w.Close()
		return err
	}
	log.Println(obj.Name())
	return w.Close()
}

func (*Blazer) DownloadFile(ctx context.Context, bucket *b2.Bucket, downloads int, src, dst string) error {
	r := bucket.Object(src).NewReader(ctx)
	defer r.Close()

	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	r.ConcurrentDownloads = downloads
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
func (b *Blazer) DownloadStreamFile(ctx context.Context,
	downloads int,
	src string,
	w io.Writer) error {
	bucket := b.Buckets[0]

	r := bucket.Object(src).NewReader(ctx)
	defer r.Close()
	r.ConcurrentDownloads = downloads
	if _, err := io.Copy(w, r); err != nil {
		return err
	}
	return nil
}

func (b *Blazer) DownloadStreamFileAndBuf(ctx context.Context,
	downloads int,
	src string,
	buf *[]byte,
	w io.Writer) error {
	bucket := b.Buckets[0]

	r := bucket.Object(src).NewReader(ctx)
	defer r.Close()
	r.ConcurrentDownloads = downloads
	if _, err := io.CopyBuffer(w, r, *buf); err != nil {
		return err
	}
	return nil
}
