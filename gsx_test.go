package gsx_test

import (
	"strings"
	"testing"

	"github.com/xDarkicex/nanite-gsx"
	"github.com/xDarkicex/nanite-render"
	"github.com/xDarkicex/nanite-gsx/internal/codegen"
	"github.com/xDarkicex/nanite-gsx/internal/parser"
)

// TestE2E_RealSource parses a real .gsx source through the lexer
// and parser, then generates Go code via the ComponentContext
// target.
func TestE2E_RealSource(t *testing.T) {
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

	got, err := codegen.Generate(parsed)
	if err != nil {
		t.Fatal(err)
	}

	// Verify key patterns.
	checks := []string{
		"func RenderUserCard",
		"*render.ComponentContext",
		"children func",
		"c.WriteString",
		"if role",
		"ADMIN",
		"func RegisterUserCard",
		"gsx.Engine",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}

	t.Logf("\nGenerated output:\n%s", got)
}

// TestEngine_CaseInsensitiveLookup verifies gsx.Engine resolves
// views case-insensitively — "UserCard" and "USERCARD" both work.
func TestEngine_CaseInsensitiveLookup(t *testing.T) {
	e := gsx.New()
	e.Register("UserCard", func(c *render.ComponentContext, data any) error {
		_, err := c.WriteString("rendered")
		return err
	})

	for _, name := range []string{"UserCard", "USERCARD", "usercard", "UsErCaRd"} {
		p, err := e.Compile(nil, name)
		if err != nil {
			t.Errorf("Compile(%q): %v", name, err)
			continue
		}
		if p == nil || p.EngineData == nil {
			t.Errorf("Compile(%q): nil program or EngineData", name)
		}
	}
}

// TestE2E_ChildrenClosure verifies non-self-closing components
// generate children closures.
func TestE2E_ChildrenClosure(t *testing.T) {
	src := `@import "myapp/models"

func DashboardLayout(title string) {
    <div class="layout">
        <header>{title}</header>
        <main>
            @children
        </main>
    </div>
}`

	parsed, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}

	got, err := codegen.Generate(parsed)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "children func") {
		t.Errorf("missing children param:\n%s", got)
	}
	if !strings.Contains(got, "if children != nil { children(c) }") {
		t.Errorf("missing @children emission:\n%s", got)
	}

	t.Logf("\nGenerated output:\n%s", got)
}

// TestE2E_DynamicAttrs verifies class={expr} generates escaped
// runtime expression output.
func TestE2E_DynamicAttrs(t *testing.T) {
	src := `func Button(btnType string) {
    <button class={"btn " + btnType}>Click</button>
}`

	parsed, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}

	got, err := codegen.Generate(parsed)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, `html.EscapeString(fmt.Sprint("btn " + btnType))`) {
		t.Errorf("missing escaped dynamic attr:\n%s", got)
	}

	t.Logf("\nGenerated output:\n%s", got)
}

// TestE2E_ComponentCall verifies component call codegen.
func TestE2E_ComponentCall(t *testing.T) {
	src := `@import "myapp/models"

func UserList(users []models.User) {
    <div class="grid">
        @for _, u := range users {
            <UserCard user={u} />
        }
    </div>
}`

	parsed, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}

	got, err := codegen.Generate(parsed)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "for _, u := range users {") {
		t.Errorf("missing for loop:\n%s", got)
	}
	if !strings.Contains(got, "RenderUserCard(c, u,") {
		t.Errorf("missing direct component call:\n%s", got)
	}
	if !strings.Contains(got, "func RegisterUserList") {
		t.Errorf("missing registration:\n%s", got)
	}

	t.Logf("\nGenerated output:\n%s", got)
}
