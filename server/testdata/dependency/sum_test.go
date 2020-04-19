package dependency

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

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

type ExampleTestSuite struct {
	suite.Suite
}

func (suite *ExampleTestSuite) TestExample() {
	Sum(1, 1)
}

func (suite *ExampleTestSuite) SetupTest() {
}

func TestExampleTestSuite(t *testing.T) {
	suite.Run(t, new(ExampleTestSuite))
}

func TestCalculatorSum(t *testing.T) {
	c := newCalc()
	if c.Sum(1, 1) != 2 {
		t.Fatal("not 2")
	}
}
