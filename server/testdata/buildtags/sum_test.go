// +build example

package buildtags

import "testing"

func TestSum(t *testing.T) {
	if Sum(1, 1) != 2 {
		t.Error("not 2")
	}
}
