package main

import (
	"errors"
	"log"
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
	DoneCh chan bool
	Status string
}

func (t *Task) Process() {
	log.Println("Processing...")
	time.Sleep(time.Millisecond * 300)

}
func (m *SFTPmanager) addTask(t *Task) {
	m.tMu.Lock()
	defer m.tMu.Unlock()
	lenTm := len(m.tasksMap)
	m.tasksMap[lenTm+1] = t
}

func (m *SFTPmanager) removeTask(id int) {
	m.tMu.Lock()
	defer m.tMu.Unlock()
	if _, exists := m.tasksMap[id]; exists {
		delete(m.tasksMap, id)
	}
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
	workers  map[int]*SFTP
	tasksMap map[int]*Task
	cMu      sync.Mutex
	tMu      sync.Mutex
	closeCh  chan (*SFTP)
	doneCh   chan *SFTP
	waitCh   chan *Task
}

type SFTP struct {
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

func (m *SFTPmanager) GetUnusingWorker(t *Task) *SFTP {
	m.cMu.Lock()
	defer m.cMu.Unlock()

	log.Println("TASK")
	for i, w := range m.workers {
		if !w.isInUse {
			log.Println("Issued unusing worker id", i)
			w.currentTask = t
			return w
		}
	}

	return nil
}
func (m *SFTPmanager) GetWorker(id int) *SFTP {
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
func (m *SFTPmanager) Run() {
	for {
		select {
		case t := <-m.waitCh:
			t.ID = len(m.tasksMap) + 1
			log.Println("received task", t.ID)
			w := m.GetUnusingWorker(t)
			if w != nil {
				m.UpdateIsInUse(t.ID, true)
				m.updateTaskStatus(t.ID, TASK_STATUS_PROCESSING)
				go w.process(t, &m.doneCh)
				continue
			}
			log.Println("Not available workers")
			t.Status = TASK_STATUS_WAITING
			m.addTask(t)
			continue
		case w := <-m.doneCh:
			log.Println("DoneCh", w.currentTask.ID)
			t := w.m.getWaitingTask()
			if t == nil {
				continue
			}
			log.Println("Worker got waiting task", t.ID)
			go w.process(t, &m.doneCh)
		}
	}
}

func NewSFTPmanager(numClients int) *SFTPmanager {
	m := &SFTPmanager{
		workers:  make(map[int]*SFTP),
		doneCh:   make(chan *SFTP),
		waitCh:   make(chan *Task),
		tasksMap: make(map[int]*Task),
		closeCh:  make(chan *SFTP),
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
func newWorker(id int, m *SFTPmanager) *SFTP {
	// user := os.Getenv("SFTP_USER")
	// server := os.Getenv("SFTP_SERVER")
	// password := os.Getenv("SFTP_PASSWORD")

	// if user == "" || server == "" || password == "" {
	// 	log.Println("Missing SFTP configuration")
	// 	return nil
	// }

	// config := &ssh.ClientConfig{
	// 	User: user,
	// 	Auth: []ssh.AuthMethod{
	// 		ssh.Password(password),
	// 	},
	// 	HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	// 	Timeout:         30 * time.Second,
	// }
	s := &SFTP{
		id: id,
		// config:     config,
		// server:     server,
		maxRetries: 5,
		m:          m,
		isInUse:    false,
		sChan:      make(chan string),
	}

	return s
}
func (m *SFTPmanager) AddWorker() (*SFTP, error) {
	worker := newWorker(len(m.workers)+1, m)
	// err := worker.connect()
	// if err != nil {
	// 	return nil, err
	// }

	m.cMu.Lock()

	m.workers[worker.id] = worker
	m.cMu.Unlock()
	log.Println("<Storage> [SFTP] added a new sftp client")
	go worker.monitor(m)

	return worker, nil
}

func (m *SFTPmanager) RemoveClient(client *SFTP) {
	m.cMu.Lock()
	defer m.cMu.Unlock()

	if _, exists := m.workers[client.id]; exists {
		close(client.closeCh)
		client.ssh.Close()
		client.client.Close()
		delete(m.workers, client.id)
	}
}

func (client *SFTP) monitor(m *SFTPmanager) {
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
			// log.Println("pinging")
			// if err := client.ping(); err != nil {
			// 	log.Printf("Connection lost to %s, attempting to reconnect...", client.server)
			// 	if err := client.reconnect(); err != nil {
			// 		log.Printf("Failed to reconnect to %s: %v", client.server, err)
			// 		m.RemoveClient(client)

			// 	}
			// 	m.closeCh <- client
			// 	return
			// }
		}
	}
}

func (client *SFTP) process(t *Task, doneCh *chan *SFTP) *Task {
	t.Process()
	client.currentTask = t
	log.Println("Process mid")
	client.m.UpdateIsInUse(client.id, false)
	client.m.removeTask(t.ID)
	t.DoneCh <- true
	*doneCh <- client

	return t
}

func (c *SFTP) connect() error {
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

func (c *SFTP) reconnect() error {
	for i := 0; i < c.maxRetries; i++ {
		if err := c.connect(); err == nil {
			return nil
		}
		time.Sleep(time.Second * time.Duration(i+1))
	}
	return errors.New("max retries exceeded")
}

func (c *SFTP) ping() error {
	wd, err := c.client.Getwd()
	log.Println("ping result", wd)
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
// 	m.workers = make(map[*SFTP]*SFTP)
// }

func (s *SFTP) Close() {
	defer close(s.closeCh)
	if s.client != nil {
		s.client.Close()
	}
	if s.ssh != nil {
		s.ssh.Close()
	}
}

// func (s *SFTP) Write(src, destination string) error {

// 	client := s.client

// 	err := s.writeFile(client, src, destination)
// 	if err != nil {
// 		err := s.writeFile(client, src, destination)
// 		if err != nil {
// 			s.closeCh <- true
// 			return err
// 		}
// 	}
// 	log.Println("Write a file")
// 	return nil
// }

// func (s *SFTP) writeFile(client *sftp.Client, src, destination string) error {
// 	remoteFile, err := client.Create(destination)
// 	if err != nil {
// 		s.closeCh <- true
// 		return err
// 	}

// 	defer remoteFile.Close()

// 	localFile, err := os.Open(src)
// 	if err != nil {
// 		s.closeCh <- true
// 		return err
// 	}
// 	defer localFile.Close()

// 	_, err = io.Copy(remoteFile, localFile)
// 	return err
// }

// func (s *SFTP) Read(w io.Writer, path string) error {

// 	err := s.readFile(s.client, w, path)
// 	if err != nil {
// 		s.closeCh <- true
// 		return err
// 	}

// 	return nil
// }

// func (s *SFTP) readFile(client *sftp.Client, w io.Writer, path string) error {
// 	remoteFile, err := client.Open(path)
// 	if err != nil {
// 		s.closeCh <- true
// 		return err
// 	}
// 	defer remoteFile.Close()

// 	_, err = io.Copy(w, remoteFile)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }
