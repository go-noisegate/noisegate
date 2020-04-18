package dependency

import "testing"

func TestSum(t *testing.T) {
	if Sum(1, 1) != 2 {
		t.Fatal("not 2")
	}
	v1 = c1
	t1 := T1{1}
	_ = t1.Inc(1)
	_ = t1.Dec(1)
	p1 := &T1{1}
	_ = p1.Inc(1)
	_ = p1.Dec(1)

	// append only
}

type ExampleTestSuite struct{}

func (suite *ExampleTestSuite) TestExample() {
	Sum(1, 1)
}

func TestExampleTestSuite(t *testing.T) {}

func TestCalculatorSum(t *testing.T) {
	c := newCalc()
	if c.Sum(1, 1) != 2 {
		t.Fatal("not 2")
	}
}
