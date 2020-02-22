package server

import (
	"testing"
	"time"
)

func TestJobProfiler(t *testing.T) {
	p := NewJobProfiler()
	p.Add("filepath", time.Microsecond)

	if e, ok := p.LastElapsedTime("filepath"); !ok || e != time.Microsecond {
		t.Errorf("wrong exec time: %v, %v", e, ok)
	}
	if _, ok := p.LastElapsedTime("/no/data"); ok {
		t.Errorf("data should not exist")
	}
}

func TestTaskProfiler(t *testing.T) {
	p := NewTaskProfiler()
	p.Add("filepath", "funcname_1", time.Microsecond)
	p.Add("filepath", "funcname_2", time.Millisecond)

	if p.ExpectExecTime("filepath", "funcname_1") != time.Microsecond {
		t.Errorf("wrong exec time: %v", p.ExpectExecTime("filepath", "funcname_1"))
	}
	if p.ExpectExecTime("filepath", "funcname_2") != time.Millisecond {
		t.Errorf("wrong exec time: %v", p.ExpectExecTime("filepath", "funcname_2"))
	}
	if p.ExpectExecTime("filepath", "funcname_3") != 0 {
		t.Errorf("exec time 0")
	}
}
