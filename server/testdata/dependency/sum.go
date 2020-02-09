package dependency

import "math"

func Sum(a, b int) int {
	return a + b
}

func NestedSum(a, b int) int {
	sum := func(a, b int) int {
		return a + b
	}
	return sum(a + b)
}

var v1 int = 1

var (
	v2 int = 1
	v3     = 1
)

const c1 = 1

type T1 struct {
	t int
}

func (t T1) Inc(a int) int {
	return a + 1
}

func MaxInt8() int {
	return math.MaxInt8
}

func (t *T1) Dec(a int) int {
	return a - 1
}

// append only
