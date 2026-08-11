# nanite-gsx

### The React/TSX template language for the nanite stack. AOT-compiled. render.Engine native.

Write components like JSX. Compile to Go functions with native `ComponentContext` access. Direct Go function calls for component composition — no string lookups, no reflection, `0 B/op` on the render path.

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
                 │  gsx compile
                 ▼
  ┌──────────────────────────────────────┐
  │  Generated Go                        │
  │                                      │
  │  func RenderUserCard(                │
  │    c *render.ComponentContext,       │
  │    user models.User,                 │
  │  ) error {                           │
  │    c.WriteString(`<div...>`)         │
  │    c.WriteString(                    │
  │      html.EscapeString(user.Name))   │
  │    if user.Admin {                   │
  │      RenderAdminBadge(c)             │
  │    }                                 │
  │    return nil                        │
  │  }                                   │
  │                                      │
  │  func RegisterUserCard(e *gsx.Engine) {  │
  │    e.Register("UserCard", ...)       │
  │  }                                   │
  └──────────────┬───────────────────────┘
                 │  implements render.Engine
                 ▼
  ┌──────────────────────────────────────┐
  │  nanite-render composition hub       │
  │  Same API as jade, templ, html/tmpl  │
  │  reg.RenderNamed(rc, "gsx", ...)     │
  │  nano.RenderPage(c, reg, ...)        │
  └──────────────┬───────────────────────┘
                 │
                 ▼
  ┌──────────────────────────────────────┐
  │  nanite router                       │
  │  190k req/s · 0 allocs per route     │
  └──────────────────────────────────────┘
```

---

## Why nanite-gsx

### The React DX, compiled to Go

`.gsx` gives you the component model of JSX — capital-letter tags, `{expressions}`, `@if`/`@for` blocks — but compiles to native Go functions that take `*render.ComponentContext`. No `io.Writer` indirection. No runtime library. Every superpower is a direct method call.

```gsx
func UserList(users []models.User) {
    <div class="grid">
        @for _, u := range users {
            <UserCard user={u} />
        }
    </div>
}
```

Compiles to:

```go
func RenderUserList(c *render.ComponentContext, users []models.User) error {
    c.WriteString(`<div class="grid">`)
    for _, u := range users {
        if err := RenderUserCard(c, u); err != nil { return err }
    }
    c.WriteString(`</div>`)
    return nil
}
```

`<UserCard user={u} />` → `RenderUserCard(c, u)` — a **direct Go function call**. Type-checked by `go build`. Zero allocations. Zero registry lookups.

### A render.Engine — like every other template language

`gsx.Engine` implements `render.Engine`. It plugs into nanite-render's composition hub exactly like `engine.Jade`, `engine.HTMLTemplate`, and `engine.HTML`:

```go
gsxEngine := gsx.New()
views.RegisterDashboard(gsxEngine)
views.RegisterHeader(gsxEngine)

reg := render.New(render.WithEngine(gsxEngine))

// Same API for every engine — gsx, jade, templ, html-template:
reg.RenderNamed(rc, "gsx", "dashboard", data)
nano.RenderPage(c, reg, "layouts/app", "dashboard", data)
```

### Built for the nanite stack

nanite-gsx is the template language for [nanite](https://github.com/xDarkicex/nanite) (the router) and [nanite-render](https://github.com/xDarkicex/nanite-render) (the composition hub). Three repos, one framework:

```
nanite ─── nanite-render ─── nanite-gsx
  │              │                │
  │ routes       │ cache          │ .gsx → Go
  │ middleware   │ components     │ engine adapter
  │ HTTP         │ HTMX, state    │ direct calls
```

### Inspired by React and Next.js

| React / Next.js | nanite-gsx |
|---|---|
| `.tsx` files | `.gsx` files — one file, one component |
| JSX `<h1>{name}</h1>` | `{name}` — same syntax, HTML-escaped |
| `{condition && <Thing/>}` | `@if condition { <Thing/> }` |
| `.map()` loops | `@for _, item := range items { ... }` |
| `{props.children}` | `@children` — renders the children closure |
| dynamic `className={...}` | `class={"btn " + type}` — split into escaped runtime writes |
| `<CapitalComponent prop={v}/>` | `<UserCard user={u}/>` → `RenderUserCard(c, u)` — direct Go call |
| `<Layout><Card/></Layout>` | Non-self-closing components → children closure |
| `"use server"` mutations | nanite-render `.Action("name", fn)` — HTMX-native |
| `useId()` | `c.UseId()` — per-request, zero-alloc first 256 |
| Context / `useContext` | `c.ProvideContext` / `c.UseContext` — zero-alloc stack |
| Error Boundaries | `.ErrorBoundary(fn)` — sync + async |
| `<Suspense>` / fallback | `.Async().Fallback(fn)` — streams via HTMX OOB |
| `import { X } from "..."` | `@import { X } from "..."` — ES6-style, compiled to Go aliases |

---

## Quick start

### Installation

```bash
go get github.com/xDarkicex/nanite-gsx
```

### Write a component

```gsx
// views/user_card.gsx
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

### Generate the Go code

```bash
gsx compile ./views
```

Produces `views/user_card_gsx.go`:

```go
// Code generated by nanite-gsx. DO NOT EDIT.
package views

import (
    "fmt"
    "html"
    models "myapp/models"
)

func RenderUserCard(c *render.ComponentContext, user models.User) error {
    c.WriteString(`<div class="user-card"><h3>`)
    c.WriteString(html.EscapeString(fmt.Sprint(user.Name)))
    c.WriteString(`</h3><p>`)
    c.WriteString(html.EscapeString(fmt.Sprint(user.Email)))
    c.WriteString(`</p>`)
    if user.IsAdmin {
        c.WriteString(`<span class="badge badge-admin">ADMIN</span>`)
    }
    c.WriteString(`</div>`)
    return nil
}

func RegisterUserCard(e *gsx.Engine) {
    e.Register("UserCard", func(c *render.ComponentContext, data any) error {
        return RenderUserCard(c, data.(models.User))
    })
}
```

### Wire it into your app

```go
package main

import (
    "github.com/xDarkicex/nanite"
    "github.com/xDarkicex/nanite-render"
    "github.com/xDarkicex/nanite-render/nano"
    "github.com/xDarkicex/nanite-render/engine"
    "github.com/xDarkicex/nanite-gsx"
    "myapp/views"
)

func main() {
    gsxEngine := gsx.New()
    views.RegisterUserCard(gsxEngine)

    reg := render.New(
        render.WithEngines(gsxEngine, engine.NewJade()),
        render.WithDefaultLoader(render.NewFileLoader("./layouts", ".jade")),
    )

    r := nanite.New()
    r.Get("/users/:id", func(c *nanite.Context) {
        user := loadUser(c.Param("id"))
        nano.RenderPage(c, reg, "layouts/app", "UserCard", user)
    })
    r.Start(":3000")
}
```

---

## Composition examples

### Dual-path execution

Every `.gsx` component has two dispatch paths:

| Path | Trigger | Cost | Type safety |
|---|---|---|---|
| **Internal fast path** | `<UserCard user={u}/>` inside another `.gsx` file | `0 B/op` — direct Go call | `go build` catches mismatches |
| **External dynamic path** | `reg.RenderNamed("gsx", "UserCard", data)` | ~50ns — map lookup + type assert | Runtime assertion (correct by convention) |

Calls between `.gsx` components use the fast path. Cross-engine calls (jade layout → gsx view, handler → gsx component) use the dynamic path. Both are generated from the same source file.

### Full-stack page

Jade layout + gsx view + gsx components — three engines, one page:

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

```go
// main.go
gsxEngine := gsx.New()
views.RegisterDashboard(gsxEngine)
views.RegisterHeader(gsxEngine)
views.RegisterStatsGrid(gsxEngine)

reg := render.New(
    render.WithEngines(gsxEngine, engine.NewJade()),
    render.WithDefaultLoader(render.NewFileLoader("./layouts", ".jade")),
)

r := nanite.New()
r.Get("/dashboard", func(c *nanite.Context) {
    user := loadCurrentUser(c)
    nano.RenderPage(c, reg, "layouts/app", "Dashboard", user)
})
```

The flow: Jade renders the layout → `{{ yield }}` triggers → `gsx.Engine.Execute` calls `RenderDashboard(c, data)` → `<Header user={user}/>` calls `RenderHeader(c, user)` directly → `<StatsGrid stats={user.Stats}/>` calls `RenderStatsGrid(c, user.Stats)` directly.

### React-style children composition

Non-self-closing components collect their inner content as a children closure. Use `@children` to place it:

```gsx
// views/dashboard_layout.gsx
@import "myapp/models"

func DashboardLayout(title string) {
    <div class="layout">
        <header class="layout-header">
            <h1>{title}</h1>
        </header>
        <main class="layout-body">
            @children
        </main>
    </div>
}
```

```gsx
// views/dashboard.gsx
func Dashboard(user models.User) {
    <DashboardLayout title={"Welcome, " + user.Name}>
        <UserCard user={user} />
        <p>Your recent activity:</p>
        <ActivityFeed />
    </DashboardLayout>
}
```

Compiles to:

```go
// Self-closing: <UserCard user={user} />  → children = nil
RenderUserCard(c, user, nil)

// Non-self-closing: wraps the inner content in a closure
RenderDashboardLayout(c, "Welcome, Alice", func(c *render.ComponentContext) error {
    if err := RenderUserCard(c, user, nil); err != nil { return err }
    c.WriteString(`<p>Your recent activity:</p>`)
    if err := RenderActivityFeed(c, nil); err != nil { return err }
    return nil
})
```

Every generated function includes a `children func(c *render.ComponentContext) error` final param. `@children` emits `if children != nil { children(c) }`. Zero reflection, zero registry lookups — just a Go function call.

### Dynamic attributes

Expression attributes generate runtime-escaped output:

```gsx
<button class={"btn " + btnType} hx-get={"/users/" + user.ID}>
    Click
</button>
```

Compiles to:

```go
c.WriteString(`<button class="`)
c.WriteString(html.EscapeString(fmt.Sprint("btn " + btnType)))
c.WriteString(`" hx-get="`)
c.WriteString(html.EscapeString(fmt.Sprint("/users/" + user.ID)))
c.WriteString(`">Click</button>`)
```

Static attributes stay as single `c.WriteString` calls. Only `{expr}` values get split into escaped runtime output. Genuinely useful for CSS class composition and HTMX URL building.

### HTMX partial swap with server actions

```gsx
// views/like_button.gsx
@import "myapp/models"

func LikeButton(post models.Post) {
    <button
        hx-post={c.ActionURL("toggle")}
        hx-swap="outerHTML"
        class="like-btn">
        {post.Likes} likes
    </button>
}
```

```go
// Registered via the generated RegisterLikeButton, mounted with:
r.Post("/_nano/action/*", reg.HandleAction)
```

The generated code has direct `c.ActionURL` access — no wrapper, no `io.Writer` proxy. The action mutates state, the component re-renders, HTMX swaps it inline. No page reload.

### Composition from a handler

```go
r.Get("/profile", func(c *nanite.Context) {
    user := loadUser(c.Param("id"))

    // RenderPage: layout + view composition
    nano.RenderPage(c, reg, "layouts/app", "Profile", user)
})

r.Get("/profile/card", func(c *nanite.Context) {
    user := loadUser(c.Param("id"))

    // RenderNamed: single gsx view, no layout
    nano.Render(c, reg, "gsx", "ProfileCard", user)
})

r.Get("/profile/badge", func(c *nanite.Context) {
    bw := render.AcquireWriter(c)
    defer render.ReleaseWriter(bw)
    rc := render.AcquireContext(bw, c.Request)
    defer render.ReleaseContext(rc)

    // Direct call — fast path, no registry
    RenderAdminBadge(rc, loadUser(c.Param("id")))
})
```

---

## Architecture

### The Engine adapter

`gsx.Engine` implements `render.Engine`. Because `.gsx` views are AOT-compiled, `Compile` is a map lookup — no runtime parsing. The view function is stored on `Program.EngineData`; `Execute` type-asserts and calls it.

```go
type Engine struct {
    views map[string]func(c *render.ComponentContext, data any) error
}

func (e *Engine) Name() string { return "gsx" }

func (e *Engine) Compile(_ []byte, name string) (*render.Program, error) {
    fn, ok := e.views[name]
    if !ok { return nil, render.ErrTemplateNotFound }
    return &render.Program{Engine: "gsx", Name: name, EngineData: fn}, nil
}

func (e *Engine) Execute(p *render.Program, w render.ByteWriter, rc *render.RenderContext, data any) error {
    fn := p.EngineData.(func(c *render.ComponentContext, data any) error)
    return fn(&render.ComponentContext{Writer: w, Context: rc, Data: data}, data)
}
```

### Compiler pipeline

```
.gsx source
    │
    ▼
┌──────────────────────┐
│  Lexer (SWAR-driven)  │  3 triggers: < { @
│  internal/lexer/      │  8 bytes/cycle, zero allocs
└──────────┬───────────┘
           │  Token stream
           ▼
┌──────────────────────┐
│  Parser               │  Token stream → NodeStream IR
│  internal/parser/     │  SoA, same pattern as nanite-render
└──────────┬───────────┘
           │  NodeStream AST
           ▼
┌──────────────────────┐
│  Code Generator       │  IR → Go source
│  internal/codegen/    │  ComponentContext target
│                       │  + RegisterX(e *gsx.Engine)
└──────────┬───────────┘
           │  _gsx.go file
           ▼
┌──────────────────────┐
│  go build             │  Type-safe, 0 B/op render
└──────────────────────┘
```

---

## The `.gsx` file format

```gsx
// ES6-style imports — compiled to Go import blocks
@import "time"
@import { User, Post } from "myapp/models"
@import db "myapp/database"

// One component per file.
// The compiler injects c *render.ComponentContext as the first param.
// Props are typed — go build catches mismatches.
func UserCard(user User, showEmail bool) {
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

    // Capital tags → direct Go function calls (fast path)
    <Avatar user={user} size="lg" />

    // Non-self-closing tags → children closure
    <DashboardLayout title="Admin">
        <UserCard user={user} />
    </DashboardLayout>

    // @children — place the children closure
    @children

    // Dynamic attributes — expressions in attribute values
    <button class={"btn " + btnType} hx-get={"/users/" + user.ID}>
}
```

**Case-insensitive lookup:** Component names are normalized — `e.Register("UserCard", fn)` registers under the lowercase key. Templates can write `<UserCard/>` (gsx convention, PascalCase) or `<USERCARD/>` (plain HTML convention, UPPERCASE) — both resolve to the same function. The source of truth is the PascalCase name in the `.gsx` file.

**Three lexer triggers:**

| Trigger | Mode | What happens |
|---|---|---|
| `<` | Tag mode | Uppercase = component call, lowercase = HTML. Non-self-closing collects inner content as children. `{attr}` values become dynamic attributes. |
| `{` | Expression mode | Balanced-brace Go expression, HTML-escaped. In attribute position (`class={expr}`), emitted as split escaped runtime output. |
| `@` | Directive mode | `@if`, `@for`, `@switch`, `@import`, `@children` |

**Imports — compiled to standard Go:**

| .gsx syntax | Generated Go |
|---|---|
| `@import "time"` | `import _ "time"` |
| `@import models "myapp/models"` | `import models "myapp/models"` |
| `@import { User, Post } from "myapp/models"` | `import pkg "myapp/models"` + `type User = pkg.User` |

---

## Status

Alpha. The prototype pipeline is complete:

- [x] Lexer (3-trigger state machine, block-detection in `scanExpr`)
- [x] Parser (`@import`, func signature, template body, `@if`/`@for`)
- [x] Codegen (`*render.ComponentContext` target, `RegisterX` wrapper)
- [x] `gsx.Engine` implementing `render.Engine`
- [x] Direct component calls (`<Card/>` → `RenderCard(c, props)`)
- [x] `@import` (3 forms)
- [x] Children closures (`<Layout><Card/></Layout>` + `@children`)
- [x] Dynamic attributes (`class={expr}` → `html.EscapeString` at runtime)
- [ ] `gsx compile` CLI
- [ ] `@switch` / `@case`
- [ ] Attribute expression values (`class={expr}`)
- [ ] `gsx watch` (hot reload)
- [ ] VS Code extension

---

## License

MIT © 2026 xDarkicex
