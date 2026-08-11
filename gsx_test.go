package gsx_test

import (
	"strings"
	"testing"

	"github.com/xDarkicex/nanite-gsx/internal/codegen"
	"github.com/xDarkicex/nanite-gsx/internal/ir"
)

// TestPrototype_Standalone generates Go source from a hand-wired
// NodeStream that represents a simple .gsx component. The IR is
// constructed middle-out — no parser yet, just verifying the
// codegen output is correct, compilable Go.
func TestPrototype_Standalone(t *testing.T) {
	b := ir.NewBuilder()

	// @view UserCard(name string, role string)
	b.SetView("UserCard",
		[]string{"name string", "role string"},
		[]string{"error"},
	)

	// <div class="card">
	b.OpenTag("div", "class", "card")

	//   <h3>{name}</h3>
	b.OpenTag("h3")
	b.AddExpr("name")
	b.CloseTag("h3")

	//   @if role == "admin" {
	b.OpenIf(`role == "admin"`)
	//     <span class="badge">ADMIN</span>
	b.OpenTag("span", "class", "badge")
	b.AddText("ADMIN")
	b.CloseTag("span")
	b.CloseControl() // }

	// </div>
	b.CloseTag("div")

	stream := b.Stream()
	// Debug: dump the IR to see tree structure.
	for i := 0; i < stream.Count; i++ {
		t.Logf("node %d: kind=%d tag=%q text=%q parent=%d firstChild=%d nextSib=%d",
			i, stream.Kind[i], stream.Tag[i], stream.Text[i],
			stream.Parent[i], stream.FirstChild[i], stream.NextSibling[i])
	}

	got, err := codegen.Standalone(stream)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the key patterns are present in the generated output.
	checks := []string{
		"package views",
		`"html"`,
		`"io"`,
		"func RenderUserCard(w io.Writer, name string, role string) (error) {",
		`w.Write([]byte(` + "`" + `<div class="card">` + "`" + `))`,
		`w.Write([]byte(` + "`" + `<h3>` + "`" + `))`,
		"io.WriteString(w, html.EscapeString(fmt.Sprint(name)))",
		`w.Write([]byte(` + "`" + `</h3>` + "`" + `))`,
		"if role == \"admin\" {",
		`w.Write([]byte(` + "`" + `<span class="badge">` + "`" + `))`,
		`w.Write([]byte(` + "`" + `ADMIN` + "`" + `))`,
		`w.Write([]byte(` + "`" + `</span>` + "`" + `))`,
		`w.Write([]byte(` + "`" + `</div>` + "`" + `))`,
		"return nil",
	}

	for i, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("check %d: missing %q\ngot:\n%s", i, want, got)
		}
	}

	t.Logf("\nGenerated output:\n%s", got)
}

// TestPrototype_NanoComponent verifies the component call
// codegen: <UserCard name={u.Name} role="admin"/> -> RenderUserCard(w, u.Name, "admin")
func TestPrototype_NanoComponent(t *testing.T) {
	b := ir.NewBuilder()
	b.SetView("UserList",
		[]string{"users []User"},
		[]string{"error"},
	)

	// <div class="list">
	b.OpenTag("div", "class", "list")
	//   @for _, u := range users {
	b.OpenFor("_, u := range users")
	//     <UserCard name={u.Name} role="admin" />
	b.AddComponent("UserCard", "name", "u.Name", "role", `"admin"`)
	b.CloseControl()
	b.CloseTag("div")

	got, err := codegen.Standalone(b.Stream())
	if err != nil {
		t.Fatal(err)
	}

	checks := []string{
		"for _, u := range users {",
		`RenderUserCard(w, name: u.Name, role: "admin")`,
	}

	for i, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("check %d: missing %q\ngot:\n%s", i, want, got)
		}
	}

	t.Logf("\nGenerated output:\n%s", got)
}
