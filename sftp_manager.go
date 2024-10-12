package main

import (
	"io"
	"log"
	"os"
	"sync"
)

type SFTPmanager struct {
	workers  map[int]*worker
	tasksMap map[int]*Task
	wMu      sync.RWMutex
	tMu      sync.RWMutex
	closeCh  chan (*worker)
	doneCh   chan *worker
	waitCh   chan *Task
}

func NewSFTPmanager(numworkers int) *SFTPmanager {
	m := &SFTPmanager{
		workers:  make(map[int]*worker),
		doneCh:   make(chan *worker),
		waitCh:   make(chan *Task),
		tMu:      sync.RWMutex{},
		tasksMap: make(map[int]*Task),
		closeCh:  make(chan *worker),
	}
	for i := 0; i < numworkers; i++ {
		w := m.addWorker()
		err := w.connect()
		if err != nil {
			err = w.reconnect()
			if err != nil {
				return nil
			}
		}
		go w.work()
		if err != nil {
			log.Println("Failed to add worker:", err)
		}
	}
	log.Println("***SFTP Manager is ready***")
	return m
}
func (m *SFTPmanager) getUnusingworker() *worker {
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
func (m *SFTPmanager) getworker(id int) *worker {
	m.wMu.Lock()
	defer m.wMu.Unlock()

	if worker, exists := m.workers[id]; exists {
		return worker
	}
	return nil
}
func (m *SFTPmanager) updateIsInUse(id int, isInUse bool) {
	m.wMu.Lock()
	defer m.wMu.Unlock()
	log.Println("Update in use: ", isInUse, "worker id:", id)
	if worker, exists := m.workers[id]; exists {
		worker.isInUse = isInUse
	}
	c := 0
	for _, uw := range m.workers {
		if uw.isInUse {
			c++
		}
	}
	log.Printf("workers in use: %d/%d", c, len(m.workers))

}

func (m *SFTPmanager) addWorker() *worker {
	worker := newWorker(len(m.workers)+1, m)
	m.wMu.Lock()

	m.workers[worker.id] = worker
	m.wMu.Unlock()
	return worker
}
func (m *SFTPmanager) removeWorker(worker *worker) {
	m.wMu.Lock()
	defer m.wMu.Unlock()
	delete(m.workers, worker.id)
	worker.close()
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
func (m *SFTPmanager) updateTaskWorker(id int, w *worker) {
	m.tMu.Lock()
	defer m.tMu.Unlock()
	log.Println("Update task's worker", id, "w id: ", w.id)
	if task, exists := m.tasksMap[id]; exists {
		task.Worker = w
	}
}

func (m *SFTPmanager) Run() {
	for {
		select {
		case t := <-m.waitCh:
			w := m.getUnusingworker()
			if w != nil {
				m.updateIsInUse(w.id, true)
				m.updateTaskStatus(t.ID, TASK_STATUS_PROCESSING)
				m.updateTaskWorker(t.ID, w)
				w.taskCh <- t
				continue
			}
			m.updateTaskStatus(t.ID, TASK_STATUS_WAITING)
			continue
		case w := <-m.doneCh:
			nt := m.getWaitingTask()
			if nt == nil {
				m.updateIsInUse(w.id, false)
				continue
			}
			m.updateIsInUse(w.id, true)
			m.updateTaskStatus(nt.ID, TASK_STATUS_PROCESSING)
			m.updateTaskWorker(nt.ID, w)
			w.taskCh <- nt
			log.Println("finish task", w.currentTask.ID, "worker: ", w.id)
		case w := <-m.closeCh:
			log.Println("error, creating a new worker, closing worker ID", w.id)
			m.updateIsInUse(w.id, true)
			go func() {
				for {
					err := w.connect()
					if err != nil {
						log.Println("Connecting againg...")
						continue
					} else {
						break
					}
				}
				go w.work()
				m.updateIsInUse(w.id, false)
			}()
		}
	}
}

func (s *worker) writeFile(w *worker, src, destination string) error {
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
