package multi_sftp

import (
	"context"
	"errors"
	"io"
	"log"
)

const (
	TASK_STATUS_INIT       = "init"
	TASK_STATUS_PROCESSING = "processing"
	TASK_STATUS_WAITING    = "waiting"
	TASK_STATUS_ERROR      = "error"
)

type processable interface {
	run(*Task) error
}
type Task struct {
	ID      int
	DoneCh  chan (*Task)
	Status  string
	Worker  *worker
	Process processable
}
type downloader struct {
	path   string
	writer io.Writer
}

func (d downloader) run(t *Task) error {
	log.Println("[MultiSFTP]<Task> download file", t.ID)

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

func (m *Manager) Download(ctx context.Context, f *File, w io.Writer) error {
	errCh := make(chan error, 1)
	go func() {
		id := len(m.tasksMap) + 1
		nt := &Task{
			ID:     id,
			DoneCh: make(chan *Task),
			Status: TASK_STATUS_INIT,
			Process: &downloader{
				path:   f.Path + f.FileName,
				writer: w,
			},
		}
		m.addTask(nt)
		defer close(nt.DoneCh)
		m.taskCh <- nt
		t := <-nt.DoneCh
		log.Println("[MultiSFTP]<Task> Success")
		m.removeTask(t.ID)
		if t.Status == TASK_STATUS_ERROR {
			errCh <- errors.New("job failed")
		}
	}()
	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err

	}
}

type uploader struct {
	path   string
	fName  string
	reader io.Reader
}

func (d uploader) run(t *Task) error {
	log.Println("[MultiSFTP]<Task> upload file", d.path)
	if err := t.Worker.sftp.MkdirAll(d.path); err != nil {
		return err
	}
	remoteFile, err := t.Worker.sftp.Create(d.path + d.fName)
	if err != nil {
		return err
	}
	defer remoteFile.Close()
	log.Println(remoteFile.Name())
	_, err = io.Copy(remoteFile, d.reader)
	if err != nil {
		return err
	}
	return nil
}

func (m *Manager) Upload(ctx context.Context, f *File, w io.Reader) error {
	log.Println("[MultiSFTP]<Task> uploading")
	errCh := make(chan error, 1)
	go func() {
		id := len(m.tasksMap) + 1
		nt := &Task{
			ID:     id,
			DoneCh: make(chan *Task),
			Status: TASK_STATUS_INIT,
			Process: &uploader{
				path:   f.Path,
				fName:  f.FileName,
				reader: w,
			},
		}
		m.addTask(nt)
		defer close(nt.DoneCh)
		m.taskCh <- nt
		t := <-nt.DoneCh
		log.Println("[MultiSFTP]<Task> DONE")
		m.removeTask(t.ID)
		if t.Status == TASK_STATUS_ERROR {
			errCh <- errors.New("job failed")
			return
		}
		errCh <- nil
	}()
	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err

	}
}

// to do
type deleter struct {
	path  string
	fName string
}

func (d deleter) run(t *Task) error {
	log.Println("[MultiSFTP]<Task> upload file", d.path)
	if err := t.Worker.sftp.MkdirAll(d.path); err != nil {
		return err
	}
	err := t.Worker.sftp.RemoveAll(d.path + d.fName)
	if err != nil {
		return err
	}

	return nil
}

func (m *Manager) Delete(ctx context.Context, path, fName string) error {
	log.Println("[MultiSFTP]<Task> uploading")
	errCh := make(chan error, 1)
	go func() {
		id := len(m.tasksMap) + 1
		nt := &Task{
			ID:     id,
			DoneCh: make(chan *Task),
			Status: TASK_STATUS_INIT,
			Process: &uploader{
				path:  path,
				fName: fName,
			},
		}
		m.addTask(nt)
		defer close(nt.DoneCh)
		m.taskCh <- nt
		t := <-nt.DoneCh
		log.Println("[MultiSFTP]<Task> DONE")
		m.removeTask(t.ID)
		if t.Status == TASK_STATUS_ERROR {
			errCh <- errors.New("job failed")
			return
		}
		errCh <- nil
	}()
	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err

	}
}
