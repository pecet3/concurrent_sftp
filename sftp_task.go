package main

import (
	"context"
	"io"
	"log"
	"sync"
	"time"
)

const (
	TASK_STATUS_INIT       = "init"
	TASK_STATUS_PROCESSING = "processing"
	TASK_STATUS_WAITING    = "waiting"
)

type Task struct {
	ID           int
	DoneCh       chan (*Task)
	Status       string
	Writer       io.Writer
	Ctx          context.Context
	Wg           sync.WaitGroup
	RetryCounter int
	Worker       *worker
}

func (t *Task) Process() {
	log.Println("Processing...")
	time.Sleep(time.Millisecond * 300)

}
func (t *Task) readFile(wr *worker, w io.Writer, path string) error {
	log.Println("reading file", t.ID)
	remoteFile, err := wr.sftp.Open(path)
	if err != nil {
		return err
	}
	defer remoteFile.Close()

	_, err = io.Copy(w, remoteFile)
	if err != nil {
		return err
	}
	return nil
}
