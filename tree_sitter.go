package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"

	_ "embed"
)

//go:embed trun_string_literal.scm
var queryTRunStringLiteral []byte

//go:embed trun_struct_literal.scm
var queryTRunStructLiteral []byte

type Predicate interface {
	Match() bool
}

type Captures map[string]*sitter.Node
type CaptureValues map[string]string

func CapturesToMap(q *sitter.Query, captures []sitter.QueryCapture) Captures {
	result := Captures{}
	for _, c := range captures {
		result[q.CaptureNameForId(c.Index)] = c.Node
	}
	return result
}

func CapturesToValues(input []byte, q *sitter.Query, captures []sitter.QueryCapture) CaptureValues {
	result := CaptureValues{}
	for _, c := range captures {
		result[q.CaptureNameForId(c.Index)] = c.Node.Content(input)
	}
	return result
}

func NewPredicate(input []byte, q *sitter.Query, m *sitter.QueryMatch) Predicate {
	predicates := q.PredicatesForPattern(uint32(m.PatternIndex))
	if len(predicates) == 0 {
		return &alwaysTruePredicate{}
	}
	captures := CapturesToMap(q, m.Captures)
	return &predicate{
		input:    input,
		q:        q,
		m:        m,
		ps:       predicates,
		captures: captures,
	}
}

type alwaysTruePredicate struct{}

func (this *alwaysTruePredicate) Match() bool { return true }

type predicate struct {
	input    []byte
	q        *sitter.Query
	m        *sitter.QueryMatch
	ps       []sitter.QueryPredicateStep
	captures map[string]*sitter.Node
}

func (this *predicate) String() string {
	var b strings.Builder
	for _, p := range this.ps {
		if b.Len() > 0 {
			b.WriteString(" ")
		}
		switch p.Type {
		case sitter.QueryPredicateStepTypeDone:
			b.WriteString("EOF")
		case sitter.QueryPredicateStepTypeString:
			b.WriteString(this.q.StringValueForId(p.ValueId))
		case sitter.QueryPredicateStepTypeCapture:
			name := this.q.CaptureNameForId(p.ValueId)
			b.WriteString(name)
			b.WriteString("(")
			b.WriteString(this.captures[name].Content(this.input))
			b.WriteString(")")
		default:
			panic(fmt.Sprintf("invalid type: %v", p.Type))
		}
	}
	return b.String()
}

func (this *predicate) Match() bool {
	//fmt.Printf("num of predicates: %d: %s\n", len(this.ps), this)
	ps := this.ps[:]
	for len(ps) > 0 {
		name := predicateMustString(this.q, ps[0])
		switch name {
		case "match?":
			capture := predicateMustCapture(this.q, ps[1])
			pattern := predicateMustString(this.q, ps[2])
			predicateMustDone(this.q, ps[3])
			value := this.captures[capture].Content(this.input)
			ok, err := regexp.MatchString(pattern, value)
			if err != nil {
				panic(err)
			}
			//fmt.Printf("match: @%v='%s' ~= '%s': %v\n", capture, value, pattern, ok)
			if !ok {
				return false
			}
			ps = ps[4:]
		case "eq?":
			capture1 := predicateMustCapture(this.q, ps[1])
			capture2 := predicateMustCapture(this.q, ps[2])
			predicateMustDone(this.q, ps[3])
			value1 := this.captures[capture1].Content(this.input)
			value2 := this.captures[capture2].Content(this.input)
			if value1 != value2 {
				return false
			}
			ps = ps[4:]
		}
	}
	return true
	//panic(fmt.Sprintf("invalid predicate: %v", name))
}

func predicateMustString(q *sitter.Query, p sitter.QueryPredicateStep) string {
	switch p.Type {
	case sitter.QueryPredicateStepTypeString:
		return q.StringValueForId(p.ValueId)
	}
	panic(fmt.Sprintf("invalid type: %v", p.Type))
}

func predicateMustCapture(q *sitter.Query, p sitter.QueryPredicateStep) string {
	switch p.Type {
	case sitter.QueryPredicateStepTypeCapture:
		return q.CaptureNameForId(p.ValueId)
	default:
		panic(fmt.Sprintf("invalid type: %v", p.Type))
	}
}

func predicateMustDone(q *sitter.Query, p sitter.QueryPredicateStep) string {
	switch p.Type {
	case sitter.QueryPredicateStepTypeDone:
		return ""
	default:
		panic(fmt.Sprintf("invalid type: %v", p.Type))
	}
}

func MustSlurp(filename string) []byte {
	f, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		panic(err)
	}
	return data
}

func Scan(input []byte, query []byte, root *sitter.Node, cb func(m *sitter.QueryMatch, captures CaptureValues)) {
	q, _ := sitter.NewQuery(query, golang.GetLanguage())
	qc := sitter.NewQueryCursor()
	qc.Exec(q, root)
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		if !NewPredicate(input, q, m).Match() {
			continue
		}
		// for _, c := range m.Captures {
		// 	fmt.Printf("- %s = %s\n", q.CaptureNameForId(c.Index), funcName(input, c.Node))
		// }
		// fmt.Println("")
		cb(m, CapturesToValues(input, q, m.Captures))
	}
}

func clear(name string) string {
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "\"", "")
	return name
}

func ScanTRunStringLiteral(input []byte, root *sitter.Node) []string {
	query := queryTRunStringLiteral
	names := []string{}
	Scan(input, query, root, func(m *sitter.QueryMatch, c CaptureValues) {
		//fmt.Printf("%v\n", captures)
		names = append(names, fmt.Sprintf("%s/%s", c["func.name"], clear(c["test.name"])))
	})
	return names
}

func ScanTRunStructLiteral(input []byte, root *sitter.Node) []string {
	query := queryTRunStructLiteral
	names := []string{}
	Scan(input, query, root, func(m *sitter.QueryMatch, c CaptureValues) {
		//fmt.Printf("%v\n", captures)
		names = append(names, fmt.Sprintf("%s/%s", c["func.name"], clear(c["test.name"])))
	})
	return names
}

func ScanTreeSitter(filename string) []string {
	input := MustSlurp(filename)
	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())
	tree := parser.Parse(nil, input)
	root := tree.RootNode()

	result := []string{}
	result = append(result, ScanTRunStringLiteral(input, root)...)
	result = append(result, ScanTRunStructLiteral(input, root)...)
	return result
}

// var query = flag.String("query", "", "filename with a query")
// var source = flag.String("source", "", "filename with a source")
//
// func main() {
// 	flag.Parse()
// 	for _, name := range ScanTreeSitter(*source) {
// 		fmt.Printf("%s\n", name)
// 	}
// }
//
// func funcName(content []byte, n *sitter.Node) string {
// 	if n == nil {
// 		return ""
// 	}
// 	return n.Content(content)
//
// 	if n.Type() != "function_declaration" {
// 		return ""
// 	}
//
// 	return n.ChildByFieldName("name").Content(content)
// }
