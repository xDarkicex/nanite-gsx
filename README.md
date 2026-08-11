# nanite-gsx

### The React/TSX-like template language for Go. AOT-compiled. Zero runtime dependencies.

Write components like JSX. Ship functions that write to `io.Writer`. No virtual DOM, no runtime library, no framework lock-in — just pure, compiled Go with `0 B/op` on the render path.

---

## Hero

```
  ┌──────────────────────────────────────┐
  │  .gsx file (your component)          │
  │                                      │
  │  func UserCard(user models.User) {   │
  │    <div class="card">                │
  │      <h3>{user.Name}</h3>            │
  │      @if user.Admin {                │
  │        <AdminBadge />                │
  │      }                               │
  │    </div>                            │
  │  }                                   │
  └──────────────┬───────────────────────┘
                 │  gsx compile ./views
                 ▼
  ┌──────────────────────────────────────┐
  │  Generated Go (zero deps)            │
  │                                      │
  │  func RenderUserCard(w io.Writer,    │
  │    user models.User) error {         │
  │    w.Write([]byte(`<div...>`))       │
  │    io.WriteString(w,                 │
  │      html.EscapeString(user.Name))   │
  │    if user.Admin {                   │
  │      RenderAdminBadge(w)             │
  │    }                                 │
  │    return nil                        │
  │  }                                   │
  └──────────────┬───────────────────────┘
                 │  works anywhere
                 ▼
  ┌──────────────────────────────────────┐
  │  net/http · nanite · chi · gin       │
  │  No runtime. No virtual DOM.         │
  └──────────────────────────────────────┘
```

---

## Why nanite-gsx

### The React DX, Go performance

`.gsx` gives you the component model of JSX — capital-letter tags, `{expressions}`, `@if`/`@for` blocks — but compiles to **zero-dependency Go functions**. No virtual DOM diffing. No runtime library. No reflection. The generated code is what you'd write by hand.

```gsx
func UserList(users []models.User) {
    <div class="grid">
        @for _, u := range users {
            <UserCard user={u} />
        }
    </div>
}
```

`<UserCard user={u} />` compiles to `RenderUserCard(w, u)` — a **direct Go function call**. The Go compiler catches type mismatches at build time. No string-based registry lookup. No reflection.

### Built for nanite-render + nanite router

`.gsx` is the template language for the [nanite](https://github.com/xDarkicex/nanite) + [nanite-render](https://github.com/xDarkicex/nanite-render) stack. When compiled with the nanite target, it auto-generates component registration wrappers:

```go
func init() {
    render.DefaultRegistry.Define("UserCard").
        Render(func(c *render.ComponentContext) error {
            props := render.BindProps[UserCardProps](c.Data)
            return RenderUserCard(c.Writer, props)
        }).
        Register(render.DefaultRegistry)
}
```

But `.gsx` works **anywhere**. The standalone target emits pure Go with `io.Writer` — use it with `net/http`, `chi`, `gin`, or just a `strings.Builder`.

### Inspired by Next.js and React

| React / Next.js | nanite-gsx |
|---|---|
| `.tsx` files | `.gsx` files — one file, one component |
| JSX expressions `<h1>{name}</h1>` | `{name}` — the same syntax |
| `{condition && <Thing/>}` | `@if condition { <Thing/> }` |
| `.map()` loops | `@for _, item := range items { ... }` |
| `<CapitalComponent prop={v}/>` | Same — direct Go function dispatch |
| `"use server"` mutations | nanite-render `.Action("name", fn)` |
| `useId()` | nanite-render `c.UseId()` |
| `import { X } from "..."` | `@import { X } from "..."` — ES6-style imports |

---

## Quick start

### Installation

```bash
go install github.com/xDarkicex/nanite-gsx/cmd/gsx@latest
```

### Write a component

```gsx
// user_card.gsx
@import "myapp/models"

func UserCard(user models.User) {
    <div class="user-card">
        <h3>{user.Name}</h3>
        <p>{user.Email}</p>
        @if user.IsAdmin {
            <span class="badge badge-admin">ADMIN</span>
        }
    </div>
}
```

### Compile

```bash
# Standalone — pure Go, works anywhere
gsx compile ./views --target=standalone

# Nanite — auto-registers with nanite-render
gsx compile ./views --target=nanite
```

Output: `user_card_gsx.go` — a `func RenderUserCard(w io.Writer, user models.User) error` that writes HTML directly to the writer.

### Use it

```go
// Standalone — no framework needed
http.HandleFunc("/users/:id", func(w http.ResponseWriter, r *http.Request) {
    user := loadUser(r.PathValue("id"))
    RenderUserCard(w, user)
})
```

---

## Examples with nanite-render + nanite router

### Full-stack page with components

```gsx
// views/header.gsx
@import "myapp/models"

func Header(user models.User) {
    <header class="app-header">
        <a href="/"><img src="/logo.svg" alt="Logo"/></a>
        <nav>
            <a href="/dashboard">Dashboard</a>
            <span class="user">{user.Name}</span>
        </nav>
    </header>
}
```

```go
// main.go
package main

import (
    "github.com/xDarkicex/nanite"
    "github.com/xDarkicex/nanite-render"
    "github.com/xDarkicex/nanite-render/nano"
    "github.com/xDarkicex/nanite-render/engine"
    "myapp/models"
    "myapp/views"
)

func main() {
    reg := render.New(
        render.WithEngines(engine.NewJade(), engine.NewHTML()),
        render.WithDefaultLoader(render.NewFileLoader("./layouts", ".jade")),
    )

    r := nanite.New()
    r.Get("/dashboard", func(c *nanite.Context) {
        user := loadCurrentUser(c)
        // nano.RenderPage: layout (jade) + view (gsx)
        if err := nano.RenderPage(c, reg, "layouts/app", "views/dashboard", user); err != nil {
            c.Error(500, err)
        }
    })
    r.Start(":3000")
}
```

### HTMX partial swap with server actions

```gsx
// views/like_button.gsx
@import "myapp/models"

func LikeButton(post models.Post) {
    <button
        hx-post="/_nano/action/LikeButton/toggle"
        hx-swap="outerHTML"
        class="like-btn">
        {post.Likes} likes
    </button>
}
```

```go
// registered via the nanite compilation target, then mounted:
r.Post("/_nano/action/*", reg.HandleAction)
```

The `.gsx` compiler generates the `func RenderLikeButton(w, post)` — nanite-render's `.Action("toggle", fn)` handles the mutation, re-renders the component, and HTMX swaps it inline. No page reload.

### Layout composition with yield

```gsx
// views/dashboard.gsx
@import "myapp/models"

func Dashboard(user models.User) {
    <Header user={user} />
    <main class="dashboard">
        <h1>Welcome, {user.Name}</h1>
        <StatsGrid stats={user.Stats} />
    </main>
}
```

```jade
// layouts/app.jade
html
  head
    NANO_HEAD
    NANO_ASSETS
  body
    {{ yield }}
```

The layout renders via jade; the view is your `.gsx` component. `NANO_HEAD` emits title/meta tags set during the render. `{{ yield }}` injects the view.

---

## The `.gsx` file format

```gsx
// Optional: ES6-style imports
@import "time"
@import { User, Post } from "myapp/models"
@import db "myapp/database"

// The component — one function per file
// @component (optional — inferred from func keyword)
func UserCard(user User, showEmail bool) error {
    // Everything from { onwards is GSX HTML mode

    // { expr } — Go expression, HTML-escaped
    <h3>{user.Name}</h3>

    // @if / @else — control flow
    @if showEmail {
        <p>{user.Email}</p>
    } @else {
        <p class="muted">Email hidden</p>
    }

    // @for — loops
    <ul>
        @for _, role := range user.Roles {
            <li class="role">{role}</li>
        }
    </ul>

    // Capital-letter tags → direct Go function calls
    <Avatar user={user} size="lg" />

    // Self-closing and HTML tags work as expected
    <br/>
    <img src={user.AvatarURL} alt="avatar"/>
}
```

**Three lexer triggers** make parsing deterministic and fast:

| Trigger | Mode | What happens |
|---|---|---|
| `<` | Tag mode | Uppercase = component call (`<UserCard/>`), lowercase = HTML (`<div>`) |
| `{` | Expression mode | Balanced-brace Go expression, HTML-escaped in output |
| `@` | Directive mode | `@if`, `@for`, `@switch`, `@import`, `@component` |

**Imports** — three forms, all compiled to standard Go `import` blocks:

| .gsx syntax | Generated Go |
|---|---|
| `@import "time"` | `import _ "time"` |
| `@import models "myapp/models"` | `import models "myapp/models"` |
| `@import { User, Post } from "myapp/models"` | `import __gsx_pkg1 "myapp/models"` + `type User = __gsx_pkg1.User` |

---

## Compilation targets

### Standalone (`--target=standalone`)

Generates pure Go — no dependencies on nanite-render. Works with any `io.Writer`:

```go
func RenderUserCard(w io.Writer, user models.User) error {
    w.Write([]byte(`<div class="user-card"><h3>`))
    io.WriteString(w, html.EscapeString(user.Name))
    w.Write([]byte(`</h3></div>`))
    return nil
}
```

### Nanite (`--target=nanite`)

Generates the standalone function **plus** automatic nanite-render component registration:

```go
func init() {
    render.DefaultRegistry.Define("UserCard").
        Render(func(c *render.ComponentContext) error {
            props := render.BindProps[UserCardProps](c.Data)
            return RenderUserCard(c.Writer, props)
        }).
        Register(render.DefaultRegistry)
}
```

Components are addressable by name from templates, `c.Render("UserCard", ...)`, and the SoA executor.

---

## Architecture

```
.gsx source
    │
    ▼
┌──────────────────────┐
│  Lexer (SWAR-driven)  │  3 triggers: < { @
│  internal/lexer/      │  Proven byte-scanning from xDarkicex/lexer
└──────────┬───────────┘
           │  Token stream
           ▼
┌──────────────────────┐
│  Parser               │  Token stream → NodeStream IR
│  internal/parser/     │  Structure-of-Arrays, like nanite-render
└──────────┬───────────┘
           │  NodeStream AST
           ▼
┌──────────────────────┐
│  Code Generator       │  IR → Go source
│  internal/codegen/    │  Recursive walker, consumed-count propagation
└──────────┬───────────┘
           │  .go file
           ▼
┌──────────────────────┐
│  go build             │  Standard Go compilation
│  (pure, zero deps)    │  0 B/op on the render path
└──────────────────────┘
```

The lexer reuses SWAR primitives from [xDarkicex/lexer](https://github.com/xDarkicex/lexer) — 8 bytes per cycle byte scanning with no allocations. The parser produces a flat `NodeStream` (same structural pattern as nanite-render's SoA executor). The code generator walks it depth-first with recursive consumed-count propagation — no node is ever emitted twice.

---

## Status

Alpha. The prototype pipeline passes end-to-end: `.gsx` source → lexer → parser → IR → Go code. Supported syntax:

- [x] Static HTML
- [x] `{ Go expressions }` — HTML-escaped
- [x] `@if` / `@else`
- [x] `@for`
- [x] `<CapitalComponent prop={val} />`
- [x] `@import` directives (3 forms)
- [x] Standalone compilation target
- [ ] Nanite compilation target
- [ ] `@switch` / `@case`
- [ ] Attribute expression values (`class={expr}`)
- [ ] VS Code extension
- [ ] `gsx watch` (hot reload)

---

## License

MIT
