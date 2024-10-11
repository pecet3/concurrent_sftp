package main

import (
	"io"
	"log"
	"os"
	"sync"
)

type SFTPmanager struct {
	workers  map[int]*Worker
	tasksMap map[int]*Task
	wMu      sync.RWMutex
	tMu      sync.RWMutex
	closeCh  chan (*Worker)
	doneCh   chan *Worker
	waitCh   chan *Task
}

func NewSFTPmanager(numworkers int) *SFTPmanager {
	m := &SFTPmanager{
		workers:  make(map[int]*Worker),
		doneCh:   make(chan *Worker),
		waitCh:   make(chan *Task),
		tMu:      sync.RWMutex{},
		tasksMap: make(map[int]*Task),
		closeCh:  make(chan *Worker),
	}
	for i := 0; i < numworkers; i++ {
		log.Println("<storage> [SFTP] Connecting...")
		w, err := m.AddWorker()
		go w.work(m)
		if err != nil {
			log.Println("Failed to add worker:", err)
		}
	}
	log.Println("SFTP Manager is ready")
	return m
}
func (m *SFTPmanager) GetUnusingWorker() *Worker {
	m.wMu.RLock()
	defer m.wMu.RUnlock()
	for _, uw := range m.workers {
		if !uw.isInUse {
			log.Println("Issued unusing worker")
			return uw
		}
	}
	return nil
}
func (m *SFTPmanager) GetWorker(id int) *Worker {
	m.wMu.Lock()
	defer m.wMu.Unlock()

	if worker, exists := m.workers[id]; exists {
		return worker
	}
	return nil
}
func (m *SFTPmanager) UpdateIsInUse(id int, isInUse bool) {
	m.wMu.Lock()
	defer m.wMu.Unlock()
	log.Println("Update in use: ", isInUse, "worker id:", id)
	if worker, exists := m.workers[id]; exists {
		worker.isInUse = isInUse
	}
}

func (m *SFTPmanager) AddWorker() (*Worker, error) {
	worker := newWorker(len(m.workers)+1, m)
	m.wMu.Lock()
	err := worker.connect()
	if err != nil {
		return nil, err
	}
	m.workers[worker.id] = worker
	m.wMu.Unlock()

	log.Println("<Storage> [SFTP] Worker connected SFTP")

	return worker, nil
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

func (m *SFTPmanager) RemoveWorker(worker *Worker) {
	m.wMu.Lock()
	defer m.wMu.Unlock()

	if _, exists := m.workers[worker.id]; exists {
		close(worker.taskCh)
		worker.ssh.Close()
		worker.sftp.Close()
		delete(m.workers, worker.id)
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

func (s *Worker) writeFile(w *Worker, src, destination string) error {
	remoteFile, err := w.sftp.Create(destination)
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
