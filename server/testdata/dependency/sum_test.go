package dependency

import "testing"

func TestSum(t *testing.T) {
	if Sum(1, 1) != 2 {
		t.Fatal("not 2")
	}
}
