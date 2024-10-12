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
	ID      int
	DoneCh  chan (*Task)
	Status  string
	Worker  *worker
	Process ProcessServices
}
type downloader struct {
	path   string
	writer io.Writer
}

func (d downloader) run(t *Task) error {
	log.Println("download file", t.ID)

	remoteFile, err := t.Worker.sftp.Open(d.path)
	if err != nil {
		return err
	}
	defer remoteFile.Close()

	_, err = io.Copy(d.writer, remoteFile)
	if err != nil {
		return err
	}
	return nil
}

func (m *SFTPmanager) Download(f *File, w io.Writer) {
	rand.Seed(uint64(time.Now().UnixNano()))
	nt := &Task{
		ID:     f.Id,
		DoneCh: make(chan *Task),
		Status: TASK_STATUS_INIT,
		Process: &downloader{
			path:   f.Path,
			writer: w,
		},
	}
	m.addTask(nt)
	defer close(nt.DoneCh)
	m.taskCh <- nt
	t := <-nt.DoneCh
	log.Println("Success")
	m.removeTask(t.ID)
}

type uploader struct {
	path   string
	reader io.Reader
}

func (d uploader) run(t *Task) error {
	log.Println("upload file", t.ID)
	remoteFile, err := t.Worker.sftp.Create(d.path)
	if err != nil {
		return err
	}
	defer remoteFile.Close()

	_, err = io.Copy(remoteFile, d.reader)
	if err != nil {
		return err
	}
	return nil
}

func (m *SFTPmanager) Upload(f *File, w io.Reader) {
	rand.Seed(uint64(time.Now().UnixNano()))
	nt := &Task{
		ID:     f.Id,
		DoneCh: make(chan *Task),
		Status: TASK_STATUS_INIT,
		Process: &uploader{
			path:   f.Path,
			reader: w,
		},
	}
	m.addTask(nt)
	defer close(nt.DoneCh)
	m.taskCh <- nt
	t := <-nt.DoneCh
	m.removeTask(t.ID)
}
