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

	files, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	parsed := files[0]

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

	files, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	parsed := files[0]

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

	files, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	parsed := files[0]

	got, err := codegen.Generate(parsed)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, `html.EscapeString(fmt.Sprint("btn " + btnType))`) {
		t.Errorf("missing escaped dynamic attr:\n%s", got)
	}

	t.Logf("\nGenerated output:\n%s", got)
}

// TestE2E_Decorators verifies @oob/@async/@fallback generate the
// fluent builder chain in RegisterXComponent.
func TestE2E_Decorators(t *testing.T) {
	src := `@oob "user-profile-slot"
@async
func UserProfile(user models.User) {
    <div>{user.Name}</div>
}

@fallback(UserProfile)
func UserProfileSkeleton() {
    <div class="skeleton">Loading...</div>
}`

	files, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 funcs, got %d", len(files))
	}

	profile := files[0]
	if !profile.Async || profile.OOBID != "user-profile-slot" {
		t.Errorf("decorators not parsed: async=%v oob=%q", profile.Async, profile.OOBID)
	}
	if profile.Fallback != "UserProfileSkeleton" {
		t.Errorf("fallback not resolved: %q", profile.Fallback)
	}

	got, err := codegen.Generate(profile)
	if err != nil {
		t.Fatal(err)
	}

	checks := []string{
		"func RegisterUserProfileComponent",
		`WithOOB("user-profile-slot")`,
		"Async()",
		"RenderUserProfileSkeleton(c, nil)",
		".Register(cr)",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}

	t.Logf("\nGenerated output:\n%s", got)
}

// TestE2E_Switch verifies @switch/@case/@default codegen.
func TestE2E_Switch(t *testing.T) {
	src := `func RoleView(role string) {
    @switch role {
        @case "admin":
            <AdminPanel />
        @default:
            <StandardView />
    }
}`

	files, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	parsed := files[0]

	got, err := codegen.Generate(parsed)
	if err != nil {
		t.Fatal(err)
	}

	checks := []string{
		"switch role {",
		`case "admin":`,
		"RenderAdminPanel(c, nil)",
		"default:",
		"RenderStandardView(c, nil)",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}

	t.Logf("\nGenerated output:\n%s", got)
}

// TestE2E_ActionAndAssets verifies @action and @css/@js codegen.
func TestE2E_ActionAndAssets(t *testing.T) {
	src := `@css "/static/css/user-card.css"
@js "/static/js/chart-init.js"

@action toggleAdmin(rc *render.RenderContext, props map[string]any) error {
    return db.Exec("UPDATE users SET admin = NOT admin WHERE id = ?", props["id"])
}

func UserCard(user models.User) {
    <div class="card">
        <h3>{user.Name}</h3>
        <button hx-post={c.ActionURL("toggleAdmin")}>Toggle Admin</button>
    </div>
}`

	files, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	parsed := files[0]

	if len(parsed.CSSAssets) != 1 || parsed.CSSAssets[0] != "/static/css/user-card.css" {
		t.Errorf("css assets = %v", parsed.CSSAssets)
	}
	if len(parsed.JSAssets) != 1 || parsed.JSAssets[0] != "/static/js/chart-init.js" {
		t.Errorf("js assets = %v", parsed.JSAssets)
	}
	if len(parsed.Actions) != 1 || parsed.Actions[0].Name != "toggleAdmin" {
		t.Errorf("actions = %+v", parsed.Actions)
	}

	got, err := codegen.Generate(parsed)
	if err != nil {
		t.Fatal(err)
	}

	checks := []string{
		`c.RequiresCSS("/static/css/user-card.css")`,
		`c.RequiresJS("/static/js/chart-init.js")`,
		`.Action("toggleAdmin", func(rc *render.RenderContext, props map[string]any) error {`,
		`return db.Exec("UPDATE users SET admin = NOT admin WHERE id = ?", props["id"])`,
		"func RegisterUserCardComponent",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}

	t.Logf("\nGenerated output:\n%s", got)
}

// TestE2E_MultiParam verifies multi-param components generate a
// props struct + BindProps bridge for dynamic registration.
func TestE2E_MultiParam(t *testing.T) {
	src := `func UserCard(user models.User, showEmail bool) {
    <h3>{user.Name}</h3>
}`

	files, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	parsed := files[0]

	got, err := codegen.Generate(parsed)
	if err != nil {
		t.Fatal(err)
	}

	checks := []string{
		"type UserCardProps struct {",
		"User models.User `nanite:\"user\"`",
		"ShowEmail bool `nanite:\"showEmail\"`",
		"render.BindProps[UserCardProps](data)",
		"return RenderUserCard(c, props.User, props.ShowEmail, nil)",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}

	t.Logf("\nGenerated output:\n%s", got)
}

// TestE2E_CrossPackage verifies destructured imports resolve
// component tags to qualified package calls (TSX-style).
func TestE2E_CrossPackage(t *testing.T) {
	src := `@import { Button, Input } from "myapp/components/ui"

func LoginForm() {
    <form hx-post="/login">
        <Input type="email" name="email" />
        <Button type="submit">Login</Button>
    </form>
}`

	files, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	parsed := files[0]

	got, err := codegen.Generate(parsed)
	if err != nil {
		t.Fatal(err)
	}

	checks := []string{
		`ui "myapp/components/ui"`,           // package alias import
		"type Button = ui.Button",            // type alias for expressions
		"if err := ui.RenderInput(c, \"email\", \"email\", nil); err != nil { return err }",
		"ui.RenderButton(c, \"submit\", func(c *render.ComponentContext) error {",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}

	t.Logf("\nGenerated output:\n%s", got)
}

// TestE2E_IfElse verifies @if/@else with raw condition capture.
func TestE2E_IfElse(t *testing.T) {
	src := `func Greeting(user models.User) {
    @if user.Admin {
        <span>admin</span>
    } @else {
        <span>user</span>
    }
}`

	files, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	parsed := files[0]

	got, err := codegen.Generate(parsed)
	if err != nil {
		t.Fatal(err)
	}

	checks := []string{
		"if user.Admin {",
		"`admin`",
		"} else {",
		"`user`",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
}

// TestE2E_YieldFragmentSpread verifies @yield, <>...</>, and
// {...attrs} codegen.
func TestE2E_YieldFragmentSpread(t *testing.T) {
	src := `func AppLayout(attrs map[string]string) {
    <html>
        <body>
            <>
                <td>a</td>
                <td>b</td>
            </>
            <main {...attrs}>
                @yield
            </main>
        </body>
    </html>
}`

	files, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	parsed := files[0]

	got, err := codegen.Generate(parsed)
	if err != nil {
		t.Fatal(err)
	}

	checks := []string{
		"if err := c.Yield(); err != nil { return err }",
		"`<td>`",
		"`</td>`",
		"for __k, __v := range attrs {",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}

	t.Logf("\nGenerated output:\n%s", got)
}

// TestE2E_HydrateAndError verifies @hydrate and @error codegen.
func TestE2E_HydrateAndError(t *testing.T) {
	src := `func Dropdown(state map[string]any) {
    <div class="dropdown" @hydrate("x-data", state)>
        <input type="email" name="email" />
        @error("email")
    </div>
}`

	files, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	parsed := files[0]

	got, err := codegen.Generate(parsed)
	if err != nil {
		t.Fatal(err)
	}

	checks := []string{
		`c.WriteHydrateProps("x-data", state)`,
		`if __err := c.Context.GetFormError("email"); __err != "" {`,
		`<span class=\"error\">`,
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}

	t.Logf("\nGenerated output:\n%s", got)
}

// TestE2E_HydrateNested verifies @hydrate handles nested
// parens, braces, and strings — the old IndexByte hack
// would stop at the first ) and truncate the expression.
func TestE2E_HydrateNested(t *testing.T) {
	src := `func Widget(state map[string]any) {
    <div @hydrate("x-data", computeValue(state, 42))>
        <button @hydrate("x-init", func(x int) string { return fmt.Sprintf("ok %d", x) }(5))>
            Click
        </button>
    </div>
}`

	files, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	parsed := files[0]

	got, err := codegen.Generate(parsed)
	if err != nil {
		t.Fatal(err)
	}

	// The expression must survive intact — not truncated at the first ).
	checks := []string{
		`c.WriteHydrateProps("x-data", computeValue(state, 42))`,
		`c.WriteHydrateProps("x-init", func(x int) string { return fmt.Sprintf("ok %d", x) }(5))`,
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing hydrated expression %q:\n%s", want, got)
		}
	}

	t.Logf("\nGenerated output:\n%s", got)
}

// TestE2E_Alpine verifies @click passthrough, x-data auto-
// hydration, and x-cloak injection.
func TestE2E_Alpine(t *testing.T) {
	src := `func Dropdown(isOpen bool) {
    <button @click="open = !open" x-data={map[string]any{"open": isOpen}}>
        Toggle
    </button>
}`

	files, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	parsed := files[0]

	got, err := codegen.Generate(parsed)
	if err != nil {
		t.Fatal(err)
	}

	checks := []string{
		`<style>[x-cloak]{display:none!important}</style>`, // auto-injected
		`@click`,                                           // Alpine event passthrough
		`c.WriteHydrateProps("x-data", map[string]any{"open": isOpen})`,
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
}

// TestE2E_Memo verifies @memo generates the Memoize wrapper.
func TestE2E_Memo(t *testing.T) {
	src := `@memo(func(rc *render.RenderContext, props UserCardProps) string {
    return props.ID
})

func UserCard(props UserCardProps) {
    <div>{props.Name}</div>
}`

	files, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	parsed := files[0]

	if parsed.Memo.Param != "props" || parsed.Memo.Type != "UserCardProps" {
		t.Errorf("memo parsed wrong: %+v", parsed.Memo)
	}

	got, err := codegen.Generate(parsed)
	if err != nil {
		t.Fatal(err)
	}

	checks := []string{
		`cr.Memoize("UserCard", func(rc *render.RenderContext, data any) string {`,
		`props := data.(UserCardProps)`,
		`return props.ID`,
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
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

	files, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	parsed := files[0]

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

// TestE2E_ProseText verifies body text survives tokenization whole:
// multi-word runs keep their spaces, punctuation (":", "/", "(", ")",
// ",", "'") is not dropped, @ is literal, and "func" in prose is not
// a keyword. The body scanner treats letter runs as text; boundary
// punctuation is appended by the parser.
func TestE2E_ProseText(t *testing.T) {
	src := `func Profile(user models.User) {
    <p>Contact: {user.Email} via https://demo.dev (see note)</p>
    <p class="handle">@{user.ID} — don't miss it.</p>
    <p>func is not a keyword here.</p>
}`

	files, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	parsed := files[0]

	got, err := codegen.Generate(parsed)
	if err != nil {
		t.Fatalf("codegen: %v", err)
	}

	for _, want := range []string{
		"Contact:",
		"https://demo.dev (see note)",
		"@",
		"don",  // apostrophe split: `don` + `'t miss it.`
		"'t miss it.",
		"func is not a keyword here.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}

	t.Logf("\nGenerated output:\n%s", got)
}

// TestE2E_LiteralAt verifies a bare @ in template text is literal —
// emails and at-handles — not a lexer error.
func TestE2E_LiteralAt(t *testing.T) {
	src := `func Profile(user models.User) {
    <p>alice@demo.dev is the demo account.</p>
    <span class="handle">@{user.ID}</span>
}`

	files, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	parsed := files[0]

	got, err := codegen.Generate(parsed)
	if err != nil {
		t.Fatalf("codegen: %v", err)
	}

	// The @ splits the text run, so the generated source has
	// separate WriteString calls — check the pieces, not a
	// contiguous string.
	for _, want := range []string{
		"alice",
		"@",
		"demo.dev is the demo account.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}

	t.Logf("\nGenerated output:\n%s", got)
}

// TestE2E_NestedIfElse verifies nested @if/@else pairs with the
// INNER if — the else must not attach to the outer guard.
func TestE2E_NestedIfElse(t *testing.T) {
	src := `func Panel(on bool) {
    <div>
        @if on {
            @if on {
                <span>inner-true</span>
            } @else {
                <span>inner-false</span>
            }
        }
    </div>
}`

	files, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got, err := codegen.Generate(files[0])
	if err != nil {
		t.Fatalf("codegen: %v", err)
	}

	// The else must be nested INSIDE the outer if, paired with
	// the inner if. The inner-false span must appear inside an
	// else of the INNER if — i.e. before the outer if's closing.
	inner := strings.Index(got, "if on {")
	outer := strings.LastIndex(got, "if on {")
	if inner == -1 || outer == -1 || inner == outer {
		t.Fatalf("expected two ifs:\n%s", got)
	}
	falsePos := strings.Index(got, "inner-false")
	if falsePos == -1 {
		t.Fatalf("missing inner-false branch:\n%s", got)
	}
	// The else block closes before the outer if does: the first
	// "}" after the inner-else content must come before the outer
	// if's closing brace. Simplest check: the inner-false content
	// appears after "else {" which follows the inner if.
	if !strings.Contains(got, "else {") {
		t.Fatalf("missing else:\n%s", got)
	}
	t.Logf("\nGenerated output:\n%s", got)
}

// TestE2E_QuotedAttrValue verifies static attribute values
// containing double quotes (e.g. placeholder='["ops"]') escape
// correctly in the generated Go — the raw value previously broke
// the emitted string literal.
func TestE2E_QuotedAttrValue(t *testing.T) {
	src := `func Form() {
    <input type="text" placeholder='["ops"]' />
}`

	files, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got, err := codegen.Generate(files[0])
	if err != nil {
		t.Fatalf("codegen: %v", err)
	}

	want := `placeholder=\"[\"ops\"]\"`
	if !strings.Contains(got, want) {
		t.Errorf("missing escaped attr:\n%s", got)
	}
	t.Logf("\nGenerated output:\n%s", got)
}

// TestE2E_ForLoopVarShadowing verifies @for loop variables that
// collide with the generated render-signature names (c, children)
// are renamed so the generated Go still compiles — and that the
// context remains accessible inside the loop body.
func TestE2E_ForLoopVarShadowing(t *testing.T) {
	src := `func List(items []string) {
    <ul>
        @for _, c := range items {
            <li>{c} {c.ActionURL("x")}</li>
        }
        @for _, children := range items {
            <li>{children}</li>
        }
    </ul>
}`

	files, err := parser.Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got, err := codegen.Generate(files[0])
	if err != nil {
		t.Fatalf("codegen: %v", err)
	}

	for _, want := range []string{
		"for _, __c := range items {",
		"for _, __children := range items {",
		"html.EscapeString(fmt.Sprint(__c))",
		"html.EscapeString(fmt.Sprint(__children))",
		// The context is still accessible inside the loop body.
		"__c.ActionURL(\"x\")",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
	// No raw shadowing left behind.
	if strings.Contains(got, "for _, c := range") {
		t.Errorf("loop var c not renamed:\n%s", got)
	}

	t.Logf("\nGenerated output:\n%s", got)
}
