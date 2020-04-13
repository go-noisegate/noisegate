package server

import "sync"

type changeManager struct {
	m   map[string][]Change
	mtx sync.Mutex
}

// Change represents the change of some region in the file.
type Change struct {
	Filename string
	// [begin, end], inclusive
	Begin, End int
}

func newChangeManager() changeManager {
	return changeManager{m: make(map[string][]Change)}
}

// Add adds the new change.
func (m changeManager) Add(dirPath string, ch Change) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	m.m[dirPath] = append(m.m[dirPath], ch)
}

// Find finds the current change list.
func (m changeManager) Find(dirPath string) []Change {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	return m.m[dirPath]
}

// Delete deletes the current change list.
func (m changeManager) Delete(dirPath string) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	delete(m.m, dirPath)
}
