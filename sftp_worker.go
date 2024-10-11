package main

import (
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

func (w *Worker) work(m *SFTPmanager) {
	defer w.Close()
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
					m.doneCh <- w
					t.DoneCh <- t
					continue
				}
			}
			m.doneCh <- w
			t.DoneCh <- t
			continue
		case <-time.After(10 * time.Second):
			if err := w.ping(); err != nil {
				log.Printf("Connection lost to %s, attempting to reconnect...", w.server)
				if err := w.reconnect(); err != nil {
					log.Printf("Failed to reconnect to %s: %v", w.server, err)
					m.RemoveWorker(w)

				}
				m.closeCh <- w
				return
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
		time.Sleep(time.Second * time.Duration(i+1))
	}
	return errors.New("max retries exceeded")
}

func (c *Worker) ping() error {
	_, err := c.sftp.Getwd()
	return err
}

func (s *Worker) Close() {
	defer close(s.taskCh)
	if s.sftp != nil {
		s.sftp.Close()
	}
	if s.ssh != nil {
		s.ssh.Close()
	}
}
