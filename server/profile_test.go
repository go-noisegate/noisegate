package server

import (
	"testing"
	"time"
)

func TestSimpleProfiler(t *testing.T) {
	p := NewSimpleProfiler()
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
