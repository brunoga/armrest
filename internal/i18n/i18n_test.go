package i18n

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// texts is the text the app's code translates: the arguments of T, N
// and Mark that are string literals or constants, in the code under
// root.
func texts(t *testing.T, root string) map[string]string {
	t.Helper()
	found := map[string]string{}
	// consts is each directory's string constants.
	consts := map[string]map[string]string{}
	var calls []struct {
		dir  string
		args []ast.Expr
		at   token.Position
	}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		dir := filepath.Dir(path)
		if consts[dir] == nil {
			consts[dir] = map[string]string{}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.ValueSpec:
				for i, name := range n.Names {
					if i < len(n.Values) {
						if s, ok := literal(n.Values[i]); ok {
							consts[dir][name.Name] = s
						}
					}
				}
			case *ast.CallExpr:
				var name string
				switch fn := n.Fun.(type) {
				case *ast.SelectorExpr:
					if x, ok := fn.X.(*ast.Ident); ok && x.Name == "i18n" {
						name = fn.Sel.Name
					}
				}
				var args []ast.Expr
				switch name {
				case "T", "Mark":
					args = n.Args[:1]
				case "N":
					args = n.Args[:2]
				}
				if args != nil {
					calls = append(calls, struct {
						dir  string
						args []ast.Expr
						at   token.Position
					}{dir, args, fset.Position(n.Pos())})
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range calls {
		for _, a := range c.args {
			if s, ok := literal(a); ok {
				found[s] = c.at.String()
			} else if id, ok := a.(*ast.Ident); ok {
				if s, ok := consts[c.dir][id.Name]; ok {
					found[s] = c.at.String()
				}
			}
		}
	}
	return found
}

func literal(e ast.Expr) (string, bool) {
	switch e := e.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			s, err := strconv.Unquote(e.Value)
			return s, err == nil
		}
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			x, ok1 := literal(e.X)
			y, ok2 := literal(e.Y)
			return x + y, ok1 && ok2
		}
	}
	return "", false
}

var verb = regexp.MustCompile(`%[-+# 0-9.]*[a-zA-Z%]`)

// TestTranslations checks each translation has the app's every text, and
// no text the app no longer has, with the same formatting verbs in the
// same order.
func TestTranslations(t *testing.T) {
	want := texts(t, "..")
	if len(want) == 0 {
		t.Fatal("no texts found")
	}
	files, err := filepath.Glob("translations/*.json")
	if err != nil || len(files) == 0 {
		t.Fatal("no translations", err)
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]string
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for s, at := range want {
			tr, ok := got[s]
			switch {
			case !ok:
				t.Errorf("%s: no translation of %q (%s)", f, s, at)
			case !slices.Equal(verbs(s), verbs(tr)):
				t.Errorf("%s: %q has verbs %v, its translation %q has %v", f, s, verbs(s), tr, verbs(tr))
			case strings.TrimSpace(tr) == "":
				t.Errorf("%s: %q translated as nothing", f, s)
			}
		}
		for s := range got {
			if _, ok := want[s]; !ok {
				t.Errorf("%s: %q is not in the app any more", f, s)
			}
		}
	}
}

func verbs(s string) []string {
	return verb.FindAllString(s, -1)
}
