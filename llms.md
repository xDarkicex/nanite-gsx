# nanite-gsx — LLM Developer Guide

This file is written for LLM agents that auto-generate `.gsx` source
code, consume the `nanite-gsx` compiler, or produce code that
integrates with generated `.gsx` output. It distils the directive
surface, the generated Go patterns, and the sharp edges so generated
code compiles on the first try.

For the full README see [README.md](README.md).

---

## 1. Module & import paths

```
module github.com/xDarkicex/nanite-gsx

go 1.25.7
```

Dependency:

```go
import "github.com/xDarkicex/nanite-gsx"
```

nanite-gsx depends on `github.com/xDarkicex/nanite-render` for the
runtime (ComponentRegistry, RenderContext, ByteWriter).

---

## 2. Golden rule for generated code

> **nanite-gsx is an AOT compiler — not a runtime template engine.**
> `.gsx` source is parsed once at build time and emitted as regular
> Go functions. There is no `template.Parse`, no reflection at
> render time, no runtime DSL. The generated Go compiles to 0 B/op
> on the hot path.

The flow:

```
.gsx source → lexer (token stream) → parser (IR Builder) → codegen (Go source)
```

The generated Go registers components with `nanite-render`'s
`ComponentRegistry`. Your application imports the generated
package and calls `RegisterX()` at boot.

---

## 3. File anatomy

```gsx
// Preamble: @import directives (optional).
@import "myapp/models"
@import { User, Product } from "myapp/models"

// Decorators (optional, above the func).
@oob "slot-id"
@async
@fallback(SkeletonComponent)
@memo(func(rc *render.RenderContext, props UserCardProps) string {
    return props.ID
})

// Action (colocated server mutation).
@action Save(rc *render.RenderContext, props FormProps) error {
    // Go mutation logic — runs on POST /_action/Component/Save
}

func ComponentName(props Type) {
    // Template body — mix HTML tags, @directives, and {Go expressions}.
    <div class="card">
        <h3>{props.Title}</h3>
        @if props.IsAdmin {
            <AdminBadge />
        }
        @children
    </div>
}
```

Generated output per `.gsx` file:
- `func RenderComponentName(c *render.ComponentContext, props Type, children ...) error` — the render function.
- `func RegisterComponentNameComponent(cr *render.ComponentRegistry)` — registers the component, wires decorators and actions.

---

## 4. Directive reference

### 4.1 Preamble directives (above the func)

| Directive | Syntax | Generated |
|---|---|---|
| `@import` | `@import "pkg"` | Go import statement |
| `@import` named | `@import { A, B } from "pkg"` | Named import |
| `@oob` | `@oob "target-id"` | `cr.Define("X").WithOOB("target-id")` |
| `@async` | `@async` | `cr.Define("X").Async()` |
| `@fallback` | `@fallback(Skeleton)` | `cr.Define("X").Fallback(RenderSkeleton)` |
| `@memo` | `@memo(func(rc, props T) string { … })` | `cr.Memoize("X", func(rc, data) string { … })` |
| `@action` | `@action Name(rc, props) error { … }` | `cr.Define("X").Action("Name", fn)` |
| `@css` | `@css "/static/app.css"` | `c.RequiresCSS("/static/app.css")` at render start |
| `@js` | `@js "/static/app.js"` | `c.RequiresJS("/static/app.js")` at render start |

### 4.2 Template body directives (inside the func)

| Directive | Syntax | Generated |
|---|---|---|
| `@if` | `@if expr { … }` | Go `if` block with `c.WriteString` |
| `@else` | `@else { … }` | Go `else` block |
| `@for` | `@for _, v := range items { … }` | Go `for` loop |
| `@switch` | `@switch expr { … }` | Go `switch` |
| `@case` | `@case "value":` | Go `case` |
| `@default` | `@default:` | Go `default` |
| `@children` | `@children` | `c.WriteChildren()` |
| `@yield` | `@yield` | `c.Context.Yield()` — layout composition |
| `@error` | `@error("field")` | Form error boundary (see §9.2 of nanite-render llms.md) |

### 4.3 Inline attribute directives (inside HTML tags)

| Directive | Syntax | Generated |
|---|---|---|
| `@hydrate` | `<div @hydrate("x-data", state)>` | `c.WriteHydrateProps("x-data", state)` |
| `x-data` auto | `<div x-data={goExpr}>` | Auto-detected → `c.WriteHydrateProps("x-data", goExpr)` |
| `x-init` auto | `<div x-init={goExpr}>` | Auto-detected → `c.WriteHydrateProps("x-init", goExpr)` |

`@hydrate` is the explicit form (any attribute name). The `x-data` /
`x-init` auto-forms are implicit sugar — the compiler detects Alpine
attributes bound to Go expressions and emits `WriteHydrateProps`
automatically. Both produce the same zero-alloc JSON bridge.

### 4.4 HTML tag syntax

```gsx
// Static HTML — emitted as-is.
<div class="container">

// Go expression — writes the Go value.
<h1>{user.Name}</h1>

// Spread attributes — runtime key/value pairs.
<div {...attrs}>

// Spread expression (React-style): {prop} is shorthand for prop={prop}.
<button {disabled}>
// → <button disabled={disabled}>

// Dynamic attributes — Go expression as attribute value.
<div class={computeClass()}>

// Boolean attributes — bare attribute, no value.
<input type="checkbox" checked>

// Self-closing tags.
<br />
<img src="/logo.png" />

// Fragments — no wrapper element.
<>
    <span>a</span>
    <span>b</span>
</>

// Component calls — CapitalCase tag names.
<UserCard user={u} />
</UserCard>
```

---

## 5. Component model

Components are Go functions taking typed props. The compiler
generates a `RenderX` function and a `RegisterXComponent` function.

### 5.1 Simple component

```gsx
func Button(label string) {
    <button class="btn">{label}</button>
}
```

Generated:

```go
func RenderButton(c *render.ComponentContext, label string, children func(c *render.ComponentContext) error) error {
    c.WriteString(`<style>[x-cloak]{display:none!important}</style>`)
    c.WriteString(`<button class="btn">`)
    c.WriteString(html.EscapeString(fmt.Sprint(label)))
    c.WriteString(`</button>`)
    return nil
}

func RegisterButtonComponent(cr *render.ComponentRegistry) {
    cr.Define("Button").
        Render(func(c *render.ComponentContext) error {
            return RenderButton(c, c.Data.(string), nil)
        }).
        Register(cr)
}
```

### 5.2 Component with decorators

```gsx
@async
@fallback(CardSkeleton)
func UserCard(props UserCardProps) {
    <div class="card">{props.Name}</div>
}
```

Generated:

```go
func RegisterUserCardComponent(cr *render.ComponentRegistry) {
    cr.Define("UserCard").
        Async().
        Fallback(func(c *render.ComponentContext) error {
            return RenderCardSkeleton(c, nil, nil)
        }).
        Render(func(c *render.ComponentContext) error {
            return RenderUserCard(c, c.Data.(UserCardProps), nil)
        }).
        Register(cr)
}
```

### 5.3 Cross-package composition

`@import` resolves CapitalCase tags to other packages. The compiler
emits a zero-byte marker type for each imported package and the
runtime dispatches to the correct `RenderX` function.

```gsx
@import { Button } from "myapp/components/ui"

func Page(props PageProps) {
    <div>
        <Button label="Click" />
    </div>
}
```

The generated code imports the remote package, creates a marker
struct, and wires the dispatch so `<Button/>` calls
`uipkg.RenderButton()`.

---

## 6. Alpine.js integration

gsx auto-injects `[x-cloak]{display:none!important}` CSS in every
render function. Alpine `x-data` and `x-init` attributes bound to Go
expressions are auto-converted to the JSON hydration bridge:

```gsx
func Dropdown(isOpen bool) {
    <div x-data={map[string]any{"open": isOpen}} @click="open = !open">
        <span x-show="open">Content</span>
    </div>
}
```

Generated:

```go
c.WriteString(`<style>[x-cloak]{display:none!important}</style>`)
c.WriteString(`<div`)
c.WriteHydrateProps("x-data", map[string]any{"open": isOpen})
c.WriteString(` @click="open = !open"`)
c.WriteString(`>`)
// ...
```

The `@click` attribute is passed through literally — Alpine event
handlers are plain strings, not Go expressions.

---

## 7. HTMX integration

HTMX attributes (`hx-get`, `hx-post`, `hx-target`, etc.) are passed
through as literal HTML attributes. The compiler does not transform
them. Use string values for static URLs and `{expr}` for dynamic:

```gsx
<button hx-get="/api/users" hx-target="#list" hx-swap="outerHTML">
    Load Users
</button>
<button hx-get={buildURL(user.ID)} hx-target="#detail">
    Load {user.Name}
</button>
```

---

## 8. Generated code conventions

- **Every render function** begins with `c.WriteString` of the
  `[x-cloak]` CSS style tag (for Alpine).
- **`html.EscapeString`** wraps all dynamic Go expression output.
  The `{expr}` form in `.gsx` produces `html.EscapeString(fmt.Sprint(expr))`.
- **`fmt` and `html`** are auto-imported in generated code. Other
  imports come from `@import` directives.
- **Static strings** use backtick raw string literals (`` `...` ``).
- **Registration functions** are named `Register{Name}Component` and
  take `*render.ComponentRegistry`.
- **`@action`** functions are hoisted into the registration block as
  closures and wired via `cr.Define("X").Action("name", fn)`.
- **Assets** (`@css`, `@js`) emit `c.RequiresCSS(path)` /
  `c.RequiresJS(path)` as the first line of the render function.
- **x-cloak CSS** is emitted once per render function. If the app
  doesn't use Alpine, it's a single style tag (negligible).

---

## 9. Compiler internals (one paragraph each)

- **`internal/lexer`** — SWAR-driven scanner. Eight bytes per cycle
  on the hot path. Emits `Kind` tokens: tags (`KindLT` → `<...>`
  span), expressions (`KindExpr` → `{...}`), `@` directives, strings,
  and Go identifiers. The `@hydrate` inline attribute is NOT a
  lexer token — it's parsed at the attribute level inside `parseAttrs`
  (the lexer sees tags as opaque `<...>` spans).
- **`internal/parser`** — consumes the token stream, builds an IR
  `NodeStream` via the Builder. `parseAttrs` handles the attribute
  micro-grammar: `@hydrate(...)`, `{...attrs}` spreads, dynamic
  `key={expr}`, static key=value. Paren-depth tracking handles nested
  func calls inside `@hydrate` expressions.
- **`internal/ir`** — intermediate representation. `NodeStream` is a
  flat slice of nodes with `AttrKeys`, `AttrVals`, `AttrDynamic`,
  `AttrHydrate`, `AttrSpread` slices. Components are `OpenComponent`
  / `CloseComponent` nodes. Control flow is `IfBlock`, `ForBlock`,
  `SwitchBlock` nodes with child ranges.
- **`internal/codegen`** — walks the IR and emits Go source. Two
  outputs per file: the `RenderX` function and the
  `RegisterXComponent` function. The `Register` function is emitted
  only when decorators, actions, or `@memo` are present.

---

## 10. Sharp edges

1. **`@hydrate` is an attribute, not a top-level directive.**
   It lives inside `<tag ...>`. Don't use it as a standalone
   `@hydrate(...)` on its own line — that won't parse.
2. **`@error("field")` IS a top-level directive** — it appears on its
   own line in the template body, not inside a tag attribute.
3. **`x-data` / `x-init` auto-hydrate only when the value is a Go
   expression** (`x-data={goExpr}`). String values (`x-data="json"`)
   are passed through as-is.
4. **CapitalCase tag names are component dispatches.** `<Div>` calls
   a component; `<div>` emits an HTML tag. The compiler checks the
   first character.
5. **Spreads use `...attrs`** (three dots prefix) inside `{...}`
   braces. Without the dots, it's a bare expression attribute
   (React-style boolean).
6. **`@import` paths are Go import paths**, not file paths. They
   must match the module's `go.mod`.
7. **Generated code imports `fmt` and `html`** automatically. If
   your expression uses other packages, import them via `@import`.
8. **The compiler is invoked via `go run ./cmd/gsx compile -dir <path>`.**
   It walks `.gsx` files in the directory and writes `_gsx.go` output
   files alongside them.
9. **The parser does not validate Go syntax** inside expressions,
   `@if` conditions, or `@for` clauses. Invalid Go passes through
   and fails at `go build` time.

---

## 11. Quick reference cheat-sheet

```
.gsx source layout:

    @import "pkg"                        // Go imports
    @import { A, B } from "pkg"          // named imports
    @oob "slot-id"                       // OOB portal target
    @async                               // server-side streaming
    @fallback(Skeleton)                  // suspense fallback
    @memo(func(rc, props T) string {…})  // component cache keyer
    @action Name(rc, props) error {…}    // colocated server action
    @css "/static/x.css"                 // CSS dependency
    @js "/static/x.js"                   // JS dependency

    func Name(props Type) {              // component definition
        <tag static="val" {bool} {...map}>
            {goExpr}                     // dynamic Go expression
            @if cond { … } @else { … }   // conditional blocks
            @for _, v := range items { … } // loops
            @switch expr { … }           // switch
            @children                    // slot children
            @yield                       // layout composition
            @error("field")              // form error boundary
        </tag>
    }
```

Happy generating.
