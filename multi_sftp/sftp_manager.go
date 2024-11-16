package multi_sftp

import (
	"log"
	"sync"
	"time"
)

type Manager struct {
	workers  map[int]*worker
	tasksMap map[int]*Task
	wMu      sync.RWMutex
	tMu      sync.RWMutex
	closeCh  chan (*worker)
	doneCh   chan *worker
	taskCh   chan *Task
}

func newManager(numworkers int) *Manager {
	m := &Manager{
		workers:  make(map[int]*worker),
		doneCh:   make(chan *worker),
		taskCh:   make(chan *Task),
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
			log.Println("[MultiSFTP] Failed to add worker:", err)
		}
	}
	time.Sleep(50 * time.Millisecond)
	log.Println("***Multi SFTP Manager is ready***")
	return m
}
func (m *Manager) getUnusingworker() *worker {
	m.wMu.RLock()
	defer m.wMu.RUnlock()
	for _, uw := range m.workers {
		if !uw.isInUse {
			log.Println("[MultiSFTP] Issued unusing worker")
			return uw
		}
	}
	return nil
}

func (m *Manager) updateIsInUse(id int, isInUse bool) {
	m.wMu.Lock()
	defer m.wMu.Unlock()
	log.Println("[MultiSFTP] Update in use: ", isInUse, "worker id:", id)
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

func (m *Manager) addWorker() *worker {
	w := newWorker(len(m.workers)+1, m)
	m.wMu.Lock()

	m.workers[w.id] = w
	m.wMu.Unlock()
	return w
}
func (m *Manager) removeWorker(worker *worker) {
	m.wMu.Lock()
	defer m.wMu.Unlock()
	delete(m.workers, worker.id)
	worker.close()
}

func (m *Manager) addTask(t *Task) {
	log.Println("[MultiSFTP] adding task ", t.ID)
	m.tMu.Lock()
	defer m.tMu.Unlock()
	m.tasksMap[t.ID] = t
}

func (m *Manager) removeTask(id int) {
	log.Println("[MultiSFTP] Removing task ", id)
	m.tMu.Lock()
	defer m.tMu.Unlock()
	delete(m.tasksMap, id)
}
func (m *Manager) getWaitingTask() *Task {
	m.tMu.Lock()
	defer m.tMu.Unlock()
	for i, t := range m.tasksMap {
		if t.Status == TASK_STATUS_WAITING {
			log.Println("[MultiSFTP] get waiting task", i)
			return t
		}
	}
	return nil
}
func (m *Manager) updateTaskStatus(id int, status string) {
	m.tMu.Lock()
	defer m.tMu.Unlock()
	log.Println("[MultiSFTP] Update task", status, id)
	if task, exists := m.tasksMap[id]; exists {
		task.Status = status
	}
}
func (m *Manager) updateTaskWorker(id int, w *worker) {
	m.tMu.Lock()
	defer m.tMu.Unlock()
	log.Println("[MultiSFTP] Update task's worker ", id, "w id: ", w.id)
	if task, exists := m.tasksMap[id]; exists {
		task.Worker = w
	}
}
func (m *Manager) Run() {
	go func() {
		for {
			select {
			case t := <-m.taskCh:
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
				log.Printf("[MultiSFTP] Worker %d has been finished a task with id %d", w.id, w.currentTask.ID)
			case w := <-m.closeCh:
				log.Println("[MultiSFTP] error, creating a new worker, closing worker ID", w.id)
				m.updateIsInUse(w.id, true)
				go func() {
					for i := 0; i < 30; i++ {
						err := w.reconnect()
						if err != nil {
							log.Println("[MultiSFTP] Worker Connecting againg...")
							time.Sleep(time.Millisecond * 1300)
							continue
						} else {
							m.updateIsInUse(w.id, false)
							go w.work()
							return
						}
					}
					m.removeWorker(w)
				}()
			}
		}
	}()
}
