package server

import "testing"

func TestScheduler_SameDepth(t *testing.T) {
	s := taskSetScheduler{}
	taskSet1 := &TaskSet{ID: 1}
	s.Add(taskSet1, 0)
	taskSet2 := &TaskSet{ID: 2}
	s.Add(taskSet2, 0)

	next, err := s.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next.ID != taskSet1.ID {
		t.Errorf("wrong next task: %v", next)
	}

	next, err = s.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next.ID != taskSet2.ID {
		t.Errorf("wrong next task: %v", next)
	}
}

func TestScheduler_DifferentDepth(t *testing.T) {
	s := taskSetScheduler{}
	taskSet1 := &TaskSet{Tasks: []*Task{&Task{TestFunction: "f1"}}}
	s.Add(taskSet1, 1)
	taskSet2 := &TaskSet{Tasks: []*Task{&Task{TestFunction: "f2"}}}
	s.Add(taskSet2, 0)

	next, err := s.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next.ID != taskSet2.ID {
		t.Errorf("wrong next task: %v", next)
	}

	next, err = s.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next.ID != taskSet1.ID {
		t.Errorf("wrong next task: %v", next)
	}
}

func TestScheduler_Empty(t *testing.T) {
	s := taskSetScheduler{}
	_, err := s.Next()
	if err != errNoTaskSet {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestScheduler_TooLargeDepth(t *testing.T) {
	s := taskSetScheduler{}
	err := s.Add(&TaskSet{}, maxDepth)
	if err == nil {
		t.Errorf("error is not returned")
	}
}

func TestScheduler_Concurrent(t *testing.T) {
	s := taskSetScheduler{}
	numGoroutines := 10
	numIt := 10
	for i := 0; i < numGoroutines*numIt; i++ {
		s.Add(&TaskSet{ID: i}, 0)
	}

	ch := make(chan int)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < numIt; j++ {
				next, err := s.Next()
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				ch <- next.ID
			}
		}()
	}

	m := make(map[int]struct{})
	for i := 0; i < numGoroutines*numIt; i++ {
		id := <-ch
		if _, ok := m[id]; ok {
			t.Errorf("duplicate id: %d", id)
		}
		m[id] = struct{}{}
	}
}
