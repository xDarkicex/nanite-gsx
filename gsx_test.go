package gsx_test

import (
	"strings"
	"testing"

	"github.com/xDarkicex/nanite-gsx/internal/codegen"
	"github.com/xDarkicex/nanite-gsx/internal/ir"
	"github.com/xDarkicex/nanite-gsx/internal/parser"
)

// TestPrototype_Standalone generates Go source from a hand-wired
// NodeStream that represents a simple .gsx component — IR only,
// no parser yet.
func TestPrototype_Standalone(t *testing.T) {
	b := ir.NewBuilder()
	b.SetView("UserCard",
		[]string{"name string", "role string"},
		[]string{"error"},
	)

	b.OpenTag("div", "class", "card")
	b.OpenTag("h3")
	b.AddExpr("name")
	b.CloseTag("h3")
	b.OpenIf(`role == "admin"`)
	b.OpenTag("span", "class", "badge")
	b.AddText("ADMIN")
	b.CloseTag("span")
	b.CloseControl()
	b.CloseTag("div")

	got, err := codegen.Standalone(b.Stream())
	if err != nil {
		t.Fatal(err)
	}

	checks := []string{
		"func RenderUserCard(w io.Writer, name string, role string) (error) {",
		"w.Write([]byte(`<div class=\"card\">`))",
		"w.Write([]byte(`<span class=\"badge\">`))",
		"w.Write([]byte(`ADMIN`))",
		"w.Write([]byte(`</span>`))",
		"return nil",
	}
	for i, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("check %d: missing %q\ngot:\n%s", i, want, got)
		}
	}
	t.Logf("\nGenerated output:\n%s", got)
}

// TestPrototype_NanoComponent verifies component call codegen.
func TestPrototype_NanoComponent(t *testing.T) {
	b := ir.NewBuilder()
	b.SetView("UserList",
		[]string{"users []User"},
		[]string{"error"},
	)

	b.OpenTag("div", "class", "list")
	b.OpenFor("_, u := range users")
	b.AddComponent("UserCard", "name", "u.Name", "role", `"admin"`)
	b.CloseControl()
	b.CloseTag("div")

	got, err := codegen.Standalone(b.Stream())
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "for _, u := range users {") {
		t.Errorf("missing for loop:\n%s", got)
	}
	if !strings.Contains(got, `RenderUserCard(w, name: u.Name, role: "admin")`) {
		t.Errorf("missing component call:\n%s", got)
	}
	t.Logf("\nGenerated output:\n%s", got)
}

// TestEndToEnd_RealSource parses real .gsx source through the
// lexer and parser, then generates Go code.
func TestEndToEnd_RealSource(t *testing.T) {
	src := `@import "time"

func UserCard(name string, role string) {
    <div class="card">
        <h3>{name}</h3>
        @if role == "admin" {
            <span class="badge">ADMIN</span>
        }
    </div>
}`

	parsed, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(parsed.Imports) != 1 || parsed.Imports[0].Path != "time" {
		t.Errorf("imports = %+v", parsed.Imports)
	}
	if parsed.FuncName != "UserCard" {
		t.Errorf("func name = %q", parsed.FuncName)
	}

	t.Logf("parsed: %s(%s)", parsed.FuncName, strings.Join(parsed.Params, ", "))
	t.Logf("body nodes: %d", parsed.Body.Count)
	for i := 0; i < parsed.Body.Count; i++ {
		t.Logf("  node %d: kind=%d tag=%q text=%q", i, parsed.Body.Kind[i], parsed.Body.Tag[i], parsed.Body.Text[i])
	}

	got, err := codegen.Standalone(parsed.Body)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "ADMIN") || !strings.Contains(got, "if role") {
		t.Errorf("pipeline output incomplete:\n%s", got)
	}
	t.Logf("\nGenerated output:\n%s", got)
}
