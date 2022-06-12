package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func Spit(data string) string {
	tempname := path.Join(os.TempDir(), "golisttests.tmp")
	err := ioutil.WriteFile(tempname, []byte(data), 0644)
	if err != nil {
		panic(err)
	}
	return tempname
}

func MustParse(code string) *ast.File {
	tempname := path.Join(os.TempDir(), "golisttests.tmp")
	err := ioutil.WriteFile(tempname, []byte(code), 0644)
	if err != nil {
		panic(err)
	}
	fset := token.NewFileSet()
	expr, err := parser.ParseFile(fset, tempname, nil, parser.ParseComments)
	if err != nil {
		panic(err)
	}
	return expr
}

func FirstFunction(code string) *ast.FuncDecl {
	for _, decl := range MustParse(code).Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok {
			return fn
		}
	}
	panic("functions not found")
}

func TestIsSingleArgumentTestingT(t *testing.T) {
	require.True(t, IsSingleArgumentTestingT(FirstFunction(`
package test
func TestSimple(t *testing.T) {}`)))
	require.True(t, IsSingleArgumentTestingT(FirstFunction(`
package test
func NoTestSimple(t *testing.T) {}`)))
	require.True(t, IsSingleArgumentTestingT(FirstFunction(`
package test
func NoTestSimple(randomName *testing.T) {}`)))
	require.False(t, IsSingleArgumentTestingT(FirstFunction(`
package test
func TestSimple(t *testing.T, more bool) {}`)))
	require.False(t, IsSingleArgumentTestingT(FirstFunction(`
package test
func TestSimple(t *testing.B, more bool) {}`)))
}

func TestIsReceiverNoArguments(t *testing.T) {
	require.False(t, HasReceiverAndNoArguments(FirstFunction(`
package test
func TestSimple1(t *testing.T) {}`)))
	require.False(t, HasReceiverAndNoArguments(FirstFunction(`
package test
func (p *int) TestSimple2(t *testing.T) {}`)))
	require.False(t, HasReceiverAndNoArguments(FirstFunction(`
package test
func (p int) TestSimple3(t *testing.T) {}`)))
	require.True(t, HasReceiverAndNoArguments(FirstFunction(`
package test
func (p int) TestSimple4() {}`)))
	require.True(t, HasReceiverAndNoArguments(FirstFunction(`
package test
func (p *int) TestSimple5() {}`)))
	require.True(t, HasReceiverAndNoArguments(FirstFunction(`
package test
func (p * int) TestSimple6() {}`)))
}

func TestGetReceiverTypeNoStar(t *testing.T) {
	require.Equal(t, "int", GetReceiverTypeNoStar(FirstFunction(`
package test
func (p int) TestSimple4() {}`)))
	require.Equal(t, "int", GetReceiverTypeNoStar(FirstFunction(`
package test
func (p *int) TestSimple5() {}`)))
	require.Equal(t, "int", GetReceiverTypeNoStar(FirstFunction(`
package test
func (p * int) TestSimple6() {}`)))
}

func TestFindSuiteRunTypes(t *testing.T) {
	require.Equal(t,
		[]string{"aggregatorStarSuite"},
		IdentNames(FindSuiteRunTypes(FirstFunction(`
package test
func TestAggregatorStarSuite(t *testing.T) {
        rand.Seed(0)
        suite.Run(t, &aggregatorStarSuite{})
        //_ = &aggregatorStarSuite{}
}`))))
	require.Equal(t,
		[]string{},
		IdentNames(FindSuiteRunTypes(FirstFunction(`
package test
func TestAggregatorStarSuite(t *testing.T) {
        rand.Seed(0)
        //suite.Run(t, &aggregatorStarSuite{})
        //_ = &aggregatorStarSuite{}
}`))))
	require.Equal(t,
		[]string{"aggregatorStarSuite", "aggregatorSuite"},
		IdentNames(FindSuiteRunTypes(FirstFunction(`
package test
func TestAggregatorStarSuite(t *testing.T) {
        rand.Seed(0)
        suite.Run(t, &aggregatorStarSuite{})
        //_ = &aggregatorStarSuite{}
        suite.Run(t, &aggregatorSuite{})
}`))))
	require.Equal(t,
		[]string{"aggregatorStarSuite", "aggregatorSuite"},
		IdentNames(FindSuiteRunTypes(FirstFunction(`
package test
func TestAggregatorStarSuite(t *testing.T) {
        rand.Seed(0)
        suite.Run(t, &aggregatorStarSuite{})
        suite.Run(t, &aggregatorSuite{})
        //_ = &aggregatorStarSuite{}
        suite.Run(t, &aggregatorSuite{})
        suite.Run(t, &aggregatorStarSuite{})
}`))))
	require.Equal(t,
		[]string{"aggregatorStarSuite"},
		IdentNames(FindSuiteRunTypes(FirstFunction(`
package test
func TestAggregatorStarSuite(t *testing.T) {
        rand.Seed(0)
        suite.Run(t, new(aggregatorStarSuite))
        //_ = &aggregatorStarSuite{}
}`))))
}

func TestIsSuiteRunner(t *testing.T) {
	require.False(t, IsSuiteRunner(FirstFunction(`
package test
func (p int) TestSimple4() {}`)))
	require.False(t, IsSuiteRunner(FirstFunction(`
package test
func NotSuiteRunner(t *testing.T) {
	suite.Run(t, &aggregatorStarSuite{})
}`)))
	require.False(t, IsSuiteRunner(FirstFunction(`
package test
func TestSuiteRunner(t *testing.T) {
	//suite.Run(t, &aggregatorStarSuite{})
}`)))
	require.True(t, IsSuiteRunner(FirstFunction(`
package test
func TestSuiteRunner(t *testing.T) {
	suite.Run(t, &aggregatorStarSuite{})
}`)))
	require.True(t, IsSuiteRunner(FirstFunction(`
package test
func TestSuiteRunner(t *testing.T) {
	suite.Run(t, &aggregatorStarSuite{
	})
}`)))
	require.True(t, IsSuiteRunner(FirstFunction(`
package test
func TestSuiteRunner(t *testing.T) {
	suite.Run(t, &aggregatorStarSuite{
		name: "test",
	})
}`)))
	require.True(t, IsSuiteRunner(FirstFunction(`
package test
func TestSuiteRunner(t *testing.T) {
	suite.Run(t, new(aggregatorStarSuite))
}`)))
}

func TestParseTestNamesSimple(t *testing.T) {
	require.Equal(t,
		[]string{},
		ParseTestNames(Spit(`
package test
func (p int) TestSimple1() {}
`)))
	require.Equal(t,
		[]string{},
		ParseTestNames(Spit(`
package test
func TestSimple2() {}
`)))
	require.Equal(t,
		[]string{},
		ParseTestNames(Spit(`
package test
func TestSimple3(t *something.T) {}
`)))
	require.Equal(t,
		[]string{"TestSimple4"},
		ParseTestNames(Spit(`
package test
func TestSimple4(t *testing.T) {}
`)))
	require.Equal(t,
		[]string{"TestSimple5"},
		ParseTestNames(Spit(`
package test
func TestSimple5(t * testing.T) {}
`)))
}

func TestParseTestNamesSuite(t *testing.T) {
	require.Equal(t,
		[]string{
			"TestSampleSuite",
			"TestSimple1",
		},
		ParseTestNames(Spit(`
package test
func TestSimple1(t *testing.T) {}
func TestSampleSuite(t *testing.T) {
	suite.Run(t, &someType{})
}
`)))
	require.Equal(t,
		[]string{
			"TestSampleSuite",
			"TestSampleSuite/TestValidAfter1",
			"TestSampleSuite/TestValidAfter2",
			"TestSampleSuite/TestValidBefore1",
			"TestSampleSuite/TestValidBefore2",
			"TestSimple1",
		},
		ParseTestNames(Spit(`
package test
func TestSimple1(t *testing.T) {}
func (s someType) TestInvalidArgs1(t *testing.T) {}
func (s *someType) TestInvalidArgs2(t *testing.T) {}
func (s someType) IncorrectName1() {}
func (s *someType) IncorrectName2() {}
func (s someType) TestValidBefore1() {}
func (s *someType) TestValidBefore2() {}
func TestSampleSuite(t *testing.T) {
	suite.Run(t, &someType{})
}
func (s someType) TestValidAfter1() {}
func (s *someType) TestValidAfter2() {}
func (s unknownType) TestNeverRun1() {}
func (s *unknownType) TestNeverRun2() {}
`)))
	require.Equal(t,
		[]string{
			"TestSameTypeDifferentSuite",
			"TestSameTypeDifferentSuite/TestValidAfter1",
			"TestSameTypeDifferentSuite/TestValidAfter2",
			"TestSameTypeDifferentSuite/TestValidBefore1",
			"TestSameTypeDifferentSuite/TestValidBefore2",
			"TestSampleSuite",
			"TestSampleSuite/TestValidAfter1",
			"TestSampleSuite/TestValidAfter2",
			"TestSampleSuite/TestValidBefore1",
			"TestSampleSuite/TestValidBefore2",
			"TestSimple1",
		},
		ParseTestNames(Spit(`
package test
func TestSimple1(t *testing.T) {}
func (s someType) TestInvalidArgs1(t *testing.T) {}
func (s *someType) TestInvalidArgs2(t *testing.T) {}
func (s someType) IncorrectName1() {}
func (s *someType) IncorrectName2() {}
func (s someType) TestValidBefore1() {}
func (s *someType) TestValidBefore2() {}
func TestSampleSuite(t *testing.T) {
	suite.Run(t, &someType{})
}
func (s someType) TestValidAfter1() {}
func (s *someType) TestValidAfter2() {}
func (s unknownType) TestNeverRun1() {}
func (s *unknownType) TestNeverRun2() {}
func TestSameTypeDifferentSuite(t *testing.T) {
	suite.Run(t, &someType{})
}
`)))
}

func TestParseTestNamesResolveEnvTypeName(t *testing.T) {
	require.Equal(t,
		[]string{"TestWeb", "TestWeb/TestValid"},
		ParseTestNames(Spit(`
package test
type Env struct {}
func (e *Env) TestValid() {}
func TestWeb(t *testing.T) {
	suite.Run(t, &Env{}) // no resolving here
}
`)))

	require.Equal(t,
		[]string{"TestWeb", "TestWeb/TestValid"},
		ParseTestNames(Spit(`
package test
type Env struct {}
func (e *Env) TestValid() {}
func TestWeb(t *testing.T) {
	env := &Env{}
	suite.Run(t, env) // resolve when already a pointer
}
`)))

	require.Equal(t,
		[]string{"TestWeb", "TestWeb/TestValid"},
		ParseTestNames(Spit(`
package test
type Env struct {}
func (e *Env) TestValid() {}
func TestWeb(t *testing.T) {
	env := Env{}
	suite.Run(t, &env) // resolve when getting a pointer
}
`)))
}
