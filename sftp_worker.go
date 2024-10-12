package main

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type Worker struct {
	id          int
	sftp        *sftp.Client
	ssh         *ssh.Client
	config      *ssh.ClientConfig
	server      string
	isInUse     bool
	taskCh      chan *Task
	m           *SFTPmanager
	currentTask *Task
}

func newWorker(id int, m *SFTPmanager) *Worker {
	user := os.Getenv("SFTP_USER")
	server := os.Getenv("SFTP_SERVER")
	password := os.Getenv("SFTP_PASSWORD")

	if user == "" || server == "" || password == "" {
		log.Println("Missing SFTP configuration")
		return nil
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}
	s := &Worker{
		id:      id,
		config:  config,
		server:  server,
		m:       m,
		isInUse: false,
		taskCh:  make(chan *Task),
	}

	return s
}

func (w *Worker) work() {
	log.Printf("Worker ID: %d is running ", w.id)
	ticker := time.NewTicker(time.Second * 10)
	defer func() {
		ticker.Stop()
		w.close()
	}()
	for {
		select {
		case t := <-w.taskCh:
			w.currentTask = t
			log.Println("PROCESSING NEW TASK IN WORKER", t.ID)
			err := t.readFile(w, t.Writer, "/wizzard.png")
			if err != nil {
				log.Println("ERROR, Retrying...", t.ID, err)
				err = t.readFile(w, t.Writer, "/wizzard.png")
				if err != nil {
					log.Println("ERROR, Retrying 2..", t.ID, err)
					t.Ctx.Err()
					w.m.doneCh <- w
					t.DoneCh <- t
					continue
				}
			}
			w.m.doneCh <- w
			t.DoneCh <- t
			continue
		case <-ticker.C:
			if !w.isInUse {
				ctx, cancel := context.WithTimeout(context.Background(),
					time.Millisecond*3000)
				defer cancel()
				if err := w.ping(ctx); err != nil {
					if err == context.DeadlineExceeded {
						log.Printf("Worker: %d Ping timed out to %s", w.id, w.server)
					} else {
						log.Printf("Worker: %d Connection lost to %s", w.id, w.server)
					}
					w.m.closeCh <- w
					return
				}
			}
		}
	}
}
func (c *Worker) connect() error {
	var err error
	c.ssh, err = ssh.Dial("tcp", c.server, c.config)
	if err != nil {
		return err
	}

	c.sftp, err = sftp.NewClient(c.ssh)
	if err != nil {
		c.ssh.Close()
		return err
	}

	return nil
}

func (c *Worker) reconnect() error {
	for i := 0; i < 5; i++ {
		if err := c.connect(); err == nil {
			return nil
		}
	}
	return errors.New("max retries exceeded")
}

func (w *Worker) ping(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		_, err := w.sftp.Getwd()
		errCh <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (s *Worker) close() {
	defer close(s.taskCh)
	if s.sftp != nil {
		s.sftp.Close()
	}
	if s.ssh != nil {
		s.ssh.Close()
	}
}
