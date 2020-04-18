package dependency_test

import (
	"testing"

	"github.com/ks888/noisegate/server/testdata/dependency"
)

func TestXSum(t *testing.T) {
	if dependency.XSum(1, 1) != 2 {
		t.Fatal("not 2")
	}
}
