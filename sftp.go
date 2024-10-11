package main

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
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
}

func (t *Task) Process() {
	log.Println("Processing...")
	time.Sleep(time.Millisecond * 300)

}
func (m *SFTPmanager) addTask(t *Task) {
	log.Println("adding task ", t.ID)
	m.tMu.Lock()
	defer m.tMu.Unlock()
	m.tasksMap[t.ID] = t
}

func (m *SFTPmanager) removeTask(id int) {
	log.Println("Removing task ", id)
	m.tMu.Lock()
	defer m.tMu.Unlock()
	delete(m.tasksMap, id)
}
func (m *SFTPmanager) getTask(id int) *Task {
	m.tMu.Lock()
	defer m.tMu.Unlock()
	if task, exists := m.tasksMap[id]; exists {
		return task
	}
	return nil
}
func (m *SFTPmanager) getWaitingTask() *Task {
	m.tMu.Lock()
	defer m.tMu.Unlock()
	for i, t := range m.tasksMap {
		if t.Status == TASK_STATUS_WAITING {
			log.Println("get waiting task", i)
			return t
		}
	}
	return nil
}
func (m *SFTPmanager) updateTaskStatus(id int, status string) {
	m.tMu.Lock()
	defer m.tMu.Unlock()
	log.Println("Update task", status, id)
	if task, exists := m.tasksMap[id]; exists {
		task.Status = status
	}
}

type SFTPmanager struct {
	workers  map[int]*Worker
	tasksMap map[int]*Task
	cMu      sync.RWMutex
	tMu      sync.RWMutex
	closeCh  chan (*Worker)
	doneCh   chan *Worker
	waitCh   chan *Task
}

type Worker struct {
	id           int
	client       *sftp.Client
	ssh          *ssh.Client
	config       *ssh.ClientConfig
	server       string
	retryCounter int
	closeCh      chan bool
	isInUse      bool
	taskCh       chan *Task
	m            *SFTPmanager
	currentTask  *Task
}

func (m *SFTPmanager) GetUnusingWorker() *Worker {
	m.cMu.RLock()
	defer m.cMu.RUnlock()
	for _, uw := range m.workers {
		if !uw.isInUse {
			log.Println("Issued unusing worker")
			return uw
		}
	}
	return nil
}
func (m *SFTPmanager) GetWorker(id int) *Worker {
	m.cMu.Lock()
	defer m.cMu.Unlock()

	if client, exists := m.workers[id]; exists {
		return client
	}
	return nil
}
func (m *SFTPmanager) UpdateIsInUse(id int, isInUse bool) {
	m.cMu.Lock()
	defer m.cMu.Unlock()
	log.Println("Update in use: ", isInUse, "worker id:", id)
	if client, exists := m.workers[id]; exists {
		client.isInUse = isInUse
	}
}

func NewSFTPmanager(numClients int) *SFTPmanager {
	m := &SFTPmanager{
		workers:  make(map[int]*Worker),
		doneCh:   make(chan *Worker),
		waitCh:   make(chan *Task),
		tMu:      sync.RWMutex{},
		tasksMap: make(map[int]*Task),
		closeCh:  make(chan *Worker),
	}

	for i := 0; i < numClients; i++ {
		log.Println("<storage> [SFTP] Connecting...")
		w, err := m.AddWorker()
		go w.monitor(m)

		if err != nil {
			log.Println("Failed to add client:", err)
		}
	}

	return m
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
		id:           id,
		config:       config,
		server:       server,
		retryCounter: 0,
		m:            m,
		isInUse:      false,
		taskCh:       make(chan *Task),
	}

	return s
}
func (m *SFTPmanager) AddWorker() (*Worker, error) {
	m.cMu.Lock()
	worker := newWorker(len(m.workers)+1, m)
	err := worker.connect()
	if err != nil {
		return nil, err
	}
	m.workers[worker.id] = worker
	m.cMu.Unlock()

	log.Println("<Storage> [SFTP] Worker connected SFTP")

	return worker, nil
}

func (m *SFTPmanager) RemoveWorker(client *Worker) {
	m.cMu.Lock()
	defer m.cMu.Unlock()

	if _, exists := m.workers[client.id]; exists {
		close(client.closeCh)
		client.ssh.Close()
		client.client.Close()
		delete(m.workers, client.id)
	}
}

func (client *Worker) monitor(m *SFTPmanager) {
	defer client.Close()
	for {
		select {
		case t := <-client.taskCh:
			client.currentTask = t
			log.Println("PROCESSING NEW TASK IN WORKER", t.ID)
			err := t.readFile(client.client, t.Writer, "/wizzard.png")
			if err != nil {
				log.Println("ERROR, Retrying...", t.ID, err)
				err = t.readFile(client.client, t.Writer, "/wizzard.png")
				if err != nil {
					log.Println("ERROR, Retrying 2..", t.ID, err)
					t.Ctx.Err()
					m.doneCh <- client
					t.DoneCh <- t
					continue
				}
			}
			m.doneCh <- client
			t.DoneCh <- t
			continue
		case <-client.closeCh:
			log.Println("closing")
			return
		case <-time.After(10 * time.Second):
			if err := client.ping(); err != nil {
				log.Printf("Connection lost to %s, attempting to reconnect...", client.server)
				if err := client.reconnect(); err != nil {
					log.Printf("Failed to reconnect to %s: %v", client.server, err)
					m.RemoveWorker(client)

				}
				m.closeCh <- client
				return
			}
		}
	}
}
func (m *SFTPmanager) Run() {
	for {
		select {
		case t := <-m.waitCh:
			w := m.GetUnusingWorker()
			if w != nil {
				m.UpdateIsInUse(w.id, true)
				m.updateTaskStatus(t.ID, TASK_STATUS_PROCESSING)
				w.taskCh <- t
				continue
			}
			log.Println("no available worker")
			m.updateTaskStatus(t.ID, TASK_STATUS_WAITING)
			continue
		case w := <-m.doneCh:
			nt := m.getWaitingTask()
			if nt == nil {
				m.UpdateIsInUse(w.id, false)
				continue
			}
			m.UpdateIsInUse(w.id, true)
			m.updateTaskStatus(nt.ID, TASK_STATUS_PROCESSING)
			w.taskCh <- nt
			log.Println("finish task", w.currentTask.ID, "worker: ", w.id)
		}
	}
}

func (c *Worker) connect() error {
	var err error
	c.ssh, err = ssh.Dial("tcp", c.server, c.config)
	if err != nil {
		return err
	}

	c.client, err = sftp.NewClient(c.ssh)
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
	_, err := c.client.Getwd()
	return err
}

func (s *Worker) Close() {
	defer close(s.closeCh)
	if s.client != nil {
		s.client.Close()
	}
	if s.ssh != nil {
		s.ssh.Close()
	}
}

func (s *Worker) writeFile(w *Worker, src, destination string) error {
	remoteFile, err := w.client.Create(destination)
	if err != nil {
		return err
	}

	defer remoteFile.Close()

	localFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer localFile.Close()

	_, err = io.Copy(remoteFile, localFile)
	return err
}

func (t *Task) readFile(client *sftp.Client, w io.Writer, path string) error {
	log.Println("reading file", t.ID)
	remoteFile, err := client.Open(path)
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

func (client *Worker) process(t *Task, doneCh *chan *Worker) {
	defer func() {
		t.DoneCh <- t
		client.currentTask = t
		*doneCh <- client
		t.Ctx.Err()
	}()
	err := t.readFile(client.client, t.Writer, "/wizzard.png")
	if err != nil {
		log.Println("ERROR PROCESS, TASK ID", t.ID, err)
	}
}
