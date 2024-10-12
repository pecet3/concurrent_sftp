package main

import (
	"io"
	"log"
	"time"

	"golang.org/x/exp/rand"
)

type File struct {
	Id         int
	Uuid       string
	IsNsfw     bool
	IsPublic   bool
	FileName   string
	Path       string
	Url        string
	Size       int64
	Ext        string
	IsTemp     bool
	UserId     int
	ComesFrom  string
	CreatedAt  time.Time
	LastOpenAt time.Time
}
type FileServices interface {
}

const (
	TASK_STATUS_INIT       = "init"
	TASK_STATUS_PROCESSING = "processing"
	TASK_STATUS_WAITING    = "waiting"
)

type ProcessServices interface {
	run(*Task) error
}
type Task struct {
	ID           int
	DoneCh       chan (*Task)
	Status       string
	Writer       io.Writer
	RetryCounter int
	Worker       *worker
	Process      ProcessServices
}
type downloader struct {
	path string
}

func (d downloader) run(t *Task) error {
	log.Println("reading file", t.ID)
	remoteFile, err := t.Worker.sftp.Open(d.path)
	if err != nil {
		return err
	}
	defer remoteFile.Close()

	_, err = io.Copy(t.Writer, remoteFile)
	if err != nil {
		return err
	}
	return nil
}

// func (t *Task) writeFile(w *worker, src, destination string) error {
// 	remoteFile, err := w.sftp.Create(destination)
// 	if err != nil {
// 		return err
// 	}

// 	defer remoteFile.Close()

// 	localFile, err := os.Open(src)
// 	if err != nil {
// 		return err
// 	}
// 	defer localFile.Close()

// 	_, err = io.Copy(remoteFile, localFile)
// 	return err
// }

func (m *SFTPmanager) NewTask(w io.Writer) {
	rand.Seed(uint64(time.Now().UnixNano()))
	id := int(rand.Int())
	nt := &Task{
		ID:     id,
		DoneCh: make(chan *Task),
		Writer: w,
		Status: TASK_STATUS_INIT,
		Process: &downloader{
			path: "/wizzard.png",
		},
	}
	m.addTask(nt)
	defer close(nt.DoneCh)
	m.taskCh <- nt

	t := <-nt.DoneCh
	m.removeTask(t.ID)
}
