package server

import (
	"errors"
	"fmt"
	"sync"
)

const maxDepth = 10

// taskSetScheduler maintains the set of the runnable task sets.
type taskSetScheduler struct {
	runnables [maxDepth][]*TaskSet
	mtx       sync.Mutex
}

// Add adds the new task set.
func (s *taskSetScheduler) Add(set *TaskSet, depth int) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if depth >= maxDepth {
		return fmt.Errorf("too large depth: %d", depth)
	}
	s.runnables[depth] = append(s.runnables[depth], set)
	return nil
}

var errNoTaskSet = errors.New("no runnable task set")

func (s *taskSetScheduler) Next() (*TaskSet, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	for i, r := range s.runnables {
		if len(r) != 0 {
			next := r[0]
			s.runnables[i] = r[1:]
			return next, nil
		}
	}

	return nil, errNoTaskSet
}

// Size returns the total nubmer of task sets.
func (s *taskSetScheduler) Size() (total int) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	for _, r := range s.runnables {
		total += len(r)
	}
	return
}
