package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var rootPath = flag.String("root", ".", "root path to start scan for test files (*_test.go)")
var limitExecution = flag.Bool("limit", false, "enable execution limiter")
var maxFiles = flag.Int("maxFiles", 10000, "max number of files to scan")
var maxExecution = flag.Duration("maxExecution", time.Second, "max time limit for scan")

var skipIdents = map[string]bool{
	"new": true,
}

func IsTestFilename(name string) bool {
	return strings.HasSuffix(name, "_test.go")
}

func IsSingleArgumentTestingT(fn *ast.FuncDecl) bool {
	if len(fn.Type.Params.List) != 1 {
		return false
	}
	for _, param := range fn.Type.Params.List {
		if star, ok := param.Type.(*ast.StarExpr); ok {
			return "&{testing T}" == fmt.Sprintf("%s", star.X)
		}
		break
	}
	return false

}

func GetReceiverTypeNoStar(fn *ast.FuncDecl) string {
	switch t := fn.Recv.List[0].Type.(type) {
	case *ast.StarExpr:
		return fmt.Sprintf("%s", t.X)
	case *ast.Ident:
		return t.Name
	}
	panic("unknown receiver type")
}

func HasReceiver(fn *ast.FuncDecl) bool {
	return fn.Recv != nil
}

func HasReceiverAndNoArguments(fn *ast.FuncDecl) bool {
	if len(fn.Type.Params.List) != 0 {
		return false
	}
	return HasReceiver(fn)
}

func IsTestName(name string) bool {
	return strings.HasPrefix(name, "Test")
}

func IsSimpleTest(fn *ast.FuncDecl) bool {
	return IsTestName(fn.Name.Name) && !HasReceiver(fn) && IsSingleArgumentTestingT(fn)
}

func IsPossibleSuiteTest(fn *ast.FuncDecl) bool {
	return IsTestName(fn.Name.Name) && HasReceiverAndNoArguments(fn)
}

func GetFirstIdent(expr ast.Expr) *ast.Ident {
	var result *ast.Ident
	ast.Inspect(expr, func(node ast.Node) bool {
		if ident, ok := node.(*ast.Ident); ok {
			if _, ok := skipIdents[ident.Name]; ok {
				return true
			}
			if result == nil {
				result = ident
			}
		}
		return true
	})
	if result == nil {
		panic("cannot find first identifier")
	}
	return result
}

func IdentNames(idents []*ast.Ident) []string {
	result := []string{}

	for _, ident := range idents {
		result = append(result, ident.Name)
	}

	return result
}

func FindSuiteRunTypes(fn *ast.FuncDecl) []*ast.Ident {
	seen := make(map[string]bool)
	result := make([]*ast.Ident, 0)
	ast.Inspect(fn, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			if len(call.Args) != 2 {
				return true
			}
			callName := fmt.Sprintf("%s", call.Fun)
			if callName != "&{suite Run}" {
				return true
			}
			ident := GetFirstIdent(call.Args[1])
			//fmt.Printf("found call (maybe var), ident name=%v, ident at=%v\n", ident.Name, fset.Position(ident.Pos()))
			if _, ok := seen[ident.Name]; !ok {
				result = append(result, ident)
				seen[ident.Name] = true
			}
		}
		return true
	})
	return result
}

func IsSuiteRunner(fn *ast.FuncDecl) bool {
	return IsSimpleTest(fn) && len(FindSuiteRunTypes(fn)) > 0
}

type Tracker struct {
	result                       []string
	seenTests                    map[string]bool
	suiteTypesAndTestsWhoRanThem map[string]map[string]bool
}

func NewTracker() *Tracker {
	return &Tracker{
		result:                       make([]string, 0),
		seenTests:                    make(map[string]bool, 0),
		suiteTypesAndTestsWhoRanThem: make(map[string]map[string]bool, 0),
	}
}

func (t *Tracker) AddTest(name string) {
	if _, ok := t.seenTests[name]; !ok {
		t.result = append(t.result, name)
		t.seenTests[name] = true
	}
}

func (t *Tracker) SeenTests() []string {
	sort.Strings(t.result)
	return t.result
}

func (t *Tracker) SuiteRanByTest(suiteTypeName string, testName string) {
	if _, ok := t.suiteTypesAndTestsWhoRanThem[suiteTypeName]; !ok {
		t.suiteTypesAndTestsWhoRanThem[suiteTypeName] = make(map[string]bool, 0)
	}
	t.suiteTypesAndTestsWhoRanThem[suiteTypeName][testName] = true
}

func (t *Tracker) WhoRanSuiteType(suiteTypeName string) []string {
	if _, ok := t.suiteTypesAndTestsWhoRanThem[suiteTypeName]; !ok {
		return []string{}
	}
	var result []string
	for testName, _ := range t.suiteTypesAndTestsWhoRanThem[suiteTypeName] {
		result = append(result, testName)
	}
	return result
}

type TypeResolver struct {
	fset *token.FileSet
	info *types.Info
}

func NewTypeResolver(fset *token.FileSet, files ...*ast.File) *TypeResolver {
	conf := types.Config{
		Importer: importer.Default(),
		Error:    func(error) {},
	}
	info := &types.Info{
		//Defs: make(map[*ast.Ident]types.Object),
		Uses: make(map[*ast.Ident]types.Object),
	}
	_, err := conf.Check("", fset, files, info)
	if err != nil {
		// it's best effort, so ignore errors, e.g. import errors
	}

	return &TypeResolver{
		fset: fset,
		info: info,
	}
}

func (this *TypeResolver) Resolve(ident *ast.Ident) string {
	for id, obj := range this.info.Uses {
		if ident.Name == obj.Name() && ident.Pos() == id.Pos() {
			typeName := obj.Type().String()
			switch p := obj.Type().(type) {
			case *types.Pointer:
				typeName = p.Elem().String()
			default:
			}
			return typeName
		}
	}
	return ident.Name // fallback
	//return ""
}

func ParseTestNames(filename string) []string {
	result := []string{}
	result1 := []string{}
	result2 := []string{}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		result1 = ParseTestNamesGolangAST(filename)
		wg.Done()
	}()
	go func() {
		result2 = ParseTestNamesTreeSitter(filename)
		wg.Done()
	}()
	//result = append(result, ParseTestNamesGolangAST(filename)...)
	//result = append(result, ParseTestNamesTreeSitter(filename)...)
	wg.Wait()
	result = append(result, result1...)
	result = append(result, result2...)
	return result
}

func ParseTestNamesTreeSitter(filename string) []string {
	return ScanTreeSitter(filename)
}

func ParseTestNamesGolangAST(filename string) []string {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return []string{}
	}
	resolver := NewTypeResolver(fset, node)
	tracker := NewTracker()
	scan := func() {
		for _, f := range node.Decls {
			if fn, ok := f.(*ast.FuncDecl); ok {
				testName := fn.Name.Name
				if IsSimpleTest(fn) {
					tracker.AddTest(testName)
					for _, runnableSuiteTypeIdent := range FindSuiteRunTypes(fn) {
						typeName := resolver.Resolve(runnableSuiteTypeIdent)
						//fmt.Printf("resolve %v => %v\n", runnableSuiteTypeIdent.Name, typeName)
						if typeName != "" {
							tracker.SuiteRanByTest(typeName, testName)
						}
					}
				}
				if IsPossibleSuiteTest(fn) {
					receiverTypeName := GetReceiverTypeNoStar(fn)
					for _, testNameWhoRan := range tracker.WhoRanSuiteType(receiverTypeName) {
						tracker.AddTest(testNameWhoRan + "/" + testName)
					}
				}
			}
		}
	}

	scan()
	scan()
	return tracker.SeenTests()
}

type Deadliner interface {
	Tick() error
}

type Limited struct {
	expiry   time.Time
	numFiles int
}

func (l *Limited) Tick() error {
	l.numFiles--
	if l.numFiles <= 0 {
		return fmt.Errorf("number of files exceeded limit (%d)", *maxFiles)
	}
	if time.Now().After(l.expiry) {
		return fmt.Errorf("execution time exceeded limit (%s)", *maxExecution)
	}
	return nil
}

type Unlimited struct{}

func (u *Unlimited) Tick() error { return nil }

func SlicerSortUniq(input []string) []string {
	sort.Strings(input)
	seen := map[string]bool{}
	var result = make([]string, 0)
	for _, line := range input {
		if _, ok := seen[line]; !ok {
			result = append(result, line)
			seen[line] = true
		}
	}
	return result
}

func ListTestNames(root string, limit Deadliner) ([]string, error) {
	var result []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if err := limit.Tick(); err != nil {
			return err
		}
		if !IsTestFilename(path) {
			return nil
		}
		result = append(result, ParseTestNames(path)...)
		return nil
	})
	return SlicerSortUniq(result), err
}

func main() {
	flag.Parse()
	var names []string
	var err error
	if *limitExecution {
		names, err = ListTestNames(*rootPath, &Limited{time.Now().Add(*maxExecution), *maxFiles})
	} else {
		names, err = ListTestNames(*rootPath, &Unlimited{})
	}
	for _, name := range names {
		fmt.Println(name)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
