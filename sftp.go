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
	ID     int
	DoneCh chan (bool)
	Status string
	Writer io.Writer
}

func (t *Task) Process() {
	log.Println("Processing...")
	time.Sleep(time.Millisecond * 300)

}
func (m *SFTPmanager) addTask(t *Task) {
	m.tMu.Lock()
	defer m.tMu.Unlock()
	m.tasksMap[t.ID] = t
}

func (m *SFTPmanager) removeTask(id int) {
	m.tMu.Lock()
	defer m.tMu.Unlock()
	delete(m.tasksMap, id)
}
func (m *SFTPmanager) getTask(id int) *Task {
	m.cMu.Lock()
	defer m.cMu.Unlock()
	if task, exists := m.tasksMap[id]; exists {
		return task
	}
	return nil
}
func (m *SFTPmanager) getWaitingTask() *Task {
	m.cMu.Lock()
	defer m.cMu.Unlock()
	for i, t := range m.tasksMap {
		if t.Status == TASK_STATUS_WAITING {
			log.Println("get waiting task", i)

			return t
		}
	}
	return nil
}
func (m *SFTPmanager) updateTaskStatus(id int, status string) {
	m.cMu.Lock()
	defer m.cMu.Unlock()
	log.Println("Update task", status, id)
	if task, exists := m.tasksMap[id]; exists {
		task.Status = status
	}
}

type SFTPmanager struct {
	workers  map[int]*Worker
	tasksMap map[int]*Task
	cMu      sync.Mutex
	tMu      sync.Mutex
	closeCh  chan (*Worker)
	doneCh   chan *Worker
	waitCh   chan *Task
}

type Worker struct {
	id          int
	client      *sftp.Client
	ssh         *ssh.Client
	config      *ssh.ClientConfig
	server      string
	maxRetries  int
	closeCh     chan bool
	isInUse     bool
	sChan       chan string
	m           *SFTPmanager
	currentTask *Task
}

func (m *SFTPmanager) GetUnusingWorker() *Worker {
	m.cMu.Lock()
	defer m.cMu.Unlock()

	log.Println("TASK")
	for i, w := range m.workers {
		if !w.isInUse {
			log.Println("Issued unusing worker id", i)
			return w
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
	log.Println("Update in use: ", isInUse)
	if client, exists := m.workers[id]; exists {
		client.isInUse = isInUse
	}
}

func NewSFTPmanager(numClients int) *SFTPmanager {
	m := &SFTPmanager{
		workers:  make(map[int]*Worker),
		doneCh:   make(chan *Worker),
		waitCh:   make(chan *Task),
		tasksMap: make(map[int]*Task),
		closeCh:  make(chan *Worker),
	}
	log.Println("Initialized")

	var wg sync.WaitGroup

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		log.Println("<storage> [SFTP] Connecting...")

		go func() {
			wg.Done()
			_, err := m.AddWorker()
			if err != nil {
				log.Println("Failed to add client:", err)
			}
		}()
	}
	wg.Wait()
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
		id:         id,
		config:     config,
		server:     server,
		maxRetries: 5,
		m:          m,
		isInUse:    false,
		sChan:      make(chan string),
	}

	return s
}
func (m *SFTPmanager) AddWorker() (*Worker, error) {
	worker := newWorker(len(m.workers)+1, m)
	err := worker.connect()
	if err != nil {
		return nil, err
	}

	m.cMu.Lock()

	m.workers[worker.id] = worker
	m.cMu.Unlock()
	log.Println("<Storage> [SFTP] added a new sftp client")
	go worker.monitor(m)

	return worker, nil
}

func (m *SFTPmanager) RemoveClient(client *Worker) {
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
		case s := <-client.sChan:
			log.Println(s)
			log.Println("<Storage> [SFTP] close client", client)
		case <-client.closeCh:
			log.Println("")
			return
		case <-time.After(10 * time.Second):
			if err := client.ping(); err != nil {
				log.Printf("Connection lost to %s, attempting to reconnect...", client.server)
				if err := client.reconnect(); err != nil {
					log.Printf("Failed to reconnect to %s: %v", client.server, err)
					m.RemoveClient(client)

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
			t.ID = len(m.tasksMap) + 1
			log.Println("received task", t.ID)
			w := m.GetUnusingWorker()
			if w != nil {
				m.UpdateIsInUse(w.id, true)
				go w.process(t, &m.doneCh)
				continue
			}
			m.addTask(t)
			continue
		case w := <-m.doneCh:
			m.removeTask(w.currentTask.ID)
			close(w.currentTask.DoneCh)
			log.Println("DoneCh", w.currentTask.ID)
			t := w.m.getWaitingTask()
			if t == nil {
				m.UpdateIsInUse(w.id, false)
				continue
			} else {

				log.Println("Worker got waiting task", t.ID)
				go w.process(t, &m.doneCh)
				continue
			}

		}
	}
}

func (client *Worker) process(t *Task, doneCh *chan *Worker) *Task {
	_, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	err := t.readFile(client.client, t.Writer, "/wizzard.png")
	if err != nil {
		log.Println(err)
		time.Sleep(300 * time.Millisecond)
		t.readFile(client.client, t.Writer, "/wizzard.png")
	}

	client.currentTask = t
	t.DoneCh <- true
	*doneCh <- client

	return t
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
	for i := 0; i < c.maxRetries; i++ {
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

// func (m *SFTPmanager) CloseAll() {
// 	m.cMu.Lock()
// 	defer m.cMu.Unlock()

// 	for client := range m.workers {
// 		close(client.closeCh)
// 		client.ssh.Close()
// 		client.client.Close()
// 	}
// 	m.workers = make(map[*Worker]*Worker)
// }

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

// func (s *Worker) Read(w io.Writer, path string) error {

// 	err := s.readFile(s.client, w, path)
// 	if err != nil {
// 		s.closeCh <- true
// 		return err
// 	}

// 	return nil
// }

func (Task) readFile(client *sftp.Client, w io.Writer, path string) error {
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
