package server

import "sync"

type ChangeManager struct {
	m   map[string][]change
	mtx sync.Mutex
}

type change struct {
	filename string
	offset   int
}

func NewChangeManager() ChangeManager {
	return ChangeManager{m: make(map[string][]change)}
}

// Add adds the new change.
func (m ChangeManager) Add(dirPath string, ch change) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	m.m[dirPath] = append(m.m[dirPath], ch)
}

// Find finds the current change list.
func (m ChangeManager) Find(dirPath string) []change {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	return m.m[dirPath]
}

// Pop returns the current change list and remove it from the manager.
func (m ChangeManager) Pop(dirPath string) []change {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	ch := m.m[dirPath]
	delete(m.m, dirPath)
	return ch
}
