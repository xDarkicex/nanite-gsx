# nanite-gsx

### The React/TSX template language for the nanite stack. AOT-compiled. render.Engine native.

Write components like JSX. Compile to Go functions with native `ComponentContext` access. Direct Go function calls for component composition — no string lookups, no reflection, `0 B/op` on the render path.

---

## Hero

```
  ┌──────────────────────────────────────┐
  │  .gsx file (your component)          │
  │                                      │
  │  @import { User } from "myapp/models"│
  │  @css "/static/css/card.css"         │
  │  @oob "card-slot"                    │
  │  @async                              │
  │  @action toggle(user) {              │
  │    db.Exec("UPDATE ...")             │
  │  }                                   │
  │                                      │
  │  func UserCard(user User) {          │
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
  │    user User,                        │
  │  ) error {                           │
  │    c.RequiresCSS("/static/...")      │
  │    c.WriteString(`<div...>`)         │
  │    ...                               │
  │  }                                   │
  │                                      │
  │  func RegisterUserCardComponent(cr)  │
  │    cr.Define("UserCard")             │
  │      .WithOOB("card-slot")           │
  │      .Async()                        │
  │      .Action("toggle", ...)          │
  │      .Render(...)                    │
  └──────────────┬───────────────────────┘
                 │  implements render.Engine
                 ▼
  ┌──────────────────────────────────────┐
  │  nanite-render composition hub       │
  │  reg.RenderNamed(rc, "gsx", ...)     │
  │  nano.RenderPage(c, reg, ...)        │
  │  <UserCard/> from any engine         │
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
| `<>...</>` fragments | `<>...</>` — zero-byte structural boundaries |
| `{...props}` spread | `{...attrs}` — runtime loop emitting escaped `key="value"` pairs |
| Layouts (`layout.tsx`) | `@yield` — pure gsx layouts, no Jade |
| `"use server"` mutations | `@action` — colocated mutations, hoisted to `.Action()` |
| CSS/JS imports | `@css` / `@js` — compiled to `c.RequiresCSS`/`c.RequiresJS`, deduped into `<NANO_ASSETS/>` |
| `useId()` | `c.UseId()` — per-request, zero-alloc first 256 |
| Client islands (Alpine.js) | `@hydrate("x-data", state)` for any attribute + native `@click` passthrough, `x-data={go}` auto-hydration, `x-cloak` injection |
| Context / `useContext` | `c.ProvideContext` / `c.UseContext` — zero-alloc stack |
| Error Boundaries | `.ErrorBoundary(fn)` — sync + async |
| `<Suspense>` / fallback | `@async` + `@fallback(X)` — generated `Async().Fallback()` chain |
| `React.memo` | `@memo(func(rc, props) string { ... })` — cached HTML, render walk bypassed on repeated keys |
| OOB portal (`createPortal`) | `@oob "slot-id"` — generated `WithOOB()` |
| `import { X } from "..."` | `@import { X } from "..."` — symbol table resolves tags to `pkg.RenderX` |
| Component libraries (`@/components/ui`) | `@import { Button } from "myapp/components/ui"` — cross-package composition via zero-byte marker types |
| Dev server hot-reload | `GSX_DEV_MODE=1 gsx watch` — browser auto-refreshes on save (nanite router + SSE Hub) |
| VS Code language support | [nanite-gsx-vscode](https://github.com/xDarkicex/nanite-gsx-vscode) — TextMate grammar with embedded Go + HTML |

### React in Go — the full picture

Everything React does on the client, `.gsx` does on the server — compiled to direct Go calls:

| Layer | React | nanite-gsx |
|---|---|---|
| Components | JSX + props | `.gsx` + typed params (`go build` catches mismatches) |
| Composition | children, fragments, spread, libraries | `@children`, `<></>`, `{...attrs}`, `@import { X }` |
| State | `useState`, Context | `c.UseState`, `c.ProvideContext`/`c.UseContext` |
| Data fetching | Server Components, `use` | `@action` colocated mutations + HTMX |
| Async UI | `<Suspense>` + fallback | `@async` + `@fallback(X)` — streams via HTMX OOB |
| Errors | Error Boundaries | `.ErrorBoundary(fn)` |
| Routing | App Router file conventions | nanite router + `nano.RenderPage` (explicit, no magic) |
| Client UI | React DOM | Alpine.js — `@click` native, `x-data` auto-hydrated |
| Head | `generateMetadata` | `@css`/`@js` + `<NanoHead/>`/`<NanoAssets/>` |
| Dev | HMR | `gsx watch` browser live-reload |
| Rendering | Virtual DOM diffing | Direct `c.WriteString` calls — `0 B/op` |
| Editor | JSX/TSX + LSP | [nanite-gsx-vscode](https://github.com/xDarkicex/nanite-gsx-vscode) — `.gsx` TextMate grammar |

React's virtual DOM exists to make client-side updates cheap. There is no client-side DOM to update — the server emits the final HTML, HTMX swaps it, Alpine adds ephemeral interactivity. The React mental model survives; the React runtime doesn't.

---

## Quick start

### Installation

```bash
go get github.com/xDarkicex/nanite-gsx
```

### VS Code extension

```bash
# Clone into your VS Code extensions directory
git clone https://github.com/xDarkicex/nanite-gsx-vscode \
  ~/.vscode/extensions/nanite.nanite-gsx-1.0.0
```

Reload VS Code (`Cmd+Shift+P` → "Developer: Reload Window") for `.gsx` syntax highlighting — `@directives`, HTML tags, `{Go expressions}`, Alpine.js, and HTMX attributes.

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
gsx compile ./views        # one-shot compile
GSX_DEV_MODE=1 gsx watch -dir ./views   # dev server + browser live-reload
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

func RenderUserCard(c *render.ComponentContext, user models.User, children func(c *render.ComponentContext) error) error {
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
        return RenderUserCard(c, data.(models.User), nil)
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

### Full-stack page — gsx layouts, zero Jade

The entire application — layout and views — is `.gsx`. No Jade needed:

```gsx
// views/app_layout.gsx
func AppLayout() {
    <html>
        <head>
            <NanoHead />
            <NanoAssets />
        </head>
        <body>
            <Navbar />
            <main>
                @yield
            </main>
        </body>
    </html>
}
```

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

```go
// main.go
gsxEngine := gsx.New()
views.RegisterAppLayout(gsxEngine)   // layout key: "AppLayout"
views.RegisterDashboard(gsxEngine)
views.RegisterHeader(gsxEngine)
views.RegisterStatsGrid(gsxEngine)

reg := render.New(render.WithEngines(gsxEngine))

r := nanite.New()
r.Get("/dashboard", func(c *nanite.Context) {
    user := loadCurrentUser(c)
    nano.RenderPage(c, reg, "AppLayout", "Dashboard", user)
})
r.Start(":3000")
```

**The flow:** `nano.RenderPage` renders the view first (`RenderDashboard(c, user)`), stashes the bytes on the RenderContext, then renders the layout. `@yield` compiles to `c.Yield()` — nanite-render's composition hook — which writes the view body where the layout says. `<Header user={user}/>` calls `RenderHeader(c, user)` directly; `<StatsGrid stats={user.Stats}/>` calls `RenderStatsGrid(c, user.Stats)` directly. `<NanoHead/>` and `<NanoAssets/>` emit head metadata and deduplicated assets. One engine, one language, the whole page.

**Key resolution:** the layout/view names passed to `nano.RenderPage` are the **registered keys** in the gsx.Engine — by default the func names (`"AppLayout"`, `"Dashboard"`). There's no file loader at runtime; the views are compiled into the binary. Path-style keys are planned (the compiler can derive `"views/app_layout"` from the file path during the directory walk).

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

### Lifecycle decorators (Suspense, OOB, fallbacks)

`@oob`, `@async`, and `@fallback` wire the component into nanite-render's advanced lifecycles — right in the template source:

```gsx
// views/user_profile.gsx
@import "myapp/models"

@oob "user-profile-slot"
@async
func UserProfile(user models.User) {
    <div class="profile">
        <h2>{user.Name}</h2>
        <p>{user.Bio}</p>
    </div>
}

@fallback(UserProfile)
func UserProfileSkeleton() {
    <div class="skeleton">
        <div class="skeleton-line"></div>
        <div class="skeleton-line short"></div>
    </div>
}
```

The compiler generates two things:

1. The plain render functions (`RenderUserProfile(c, user)`, `RenderUserProfileSkeleton(c)`) — usable directly
2. A decorated registration that builds the full fluent chain:

```go
func RegisterUserProfileComponent(cr *render.ComponentRegistry) {
    cr.Define("UserProfile").
        WithOOB("user-profile-slot").
        Async().
        Fallback(func(c *render.ComponentContext) error {
            return RenderUserProfileSkeleton(c, nil)
        }).
        Render(func(c *render.ComponentContext) error {
            return RenderUserProfile(c, c.Data.(models.User), nil)
        }).
        Register(cr)
}
```

The skeleton renders inline instantly (TTFB ≈ 0ms); the worker goroutine renders the real profile and streams it as an HTMX OOB swap when done. One file, two functions, zero manual wiring.

### Colocated server actions (`@action`)

Mutation logic lives next to the component. The compiler hoists it into the fluent builder's `.Action()` chain — HTMX-native, router-agnostic, secure by default:

```gsx
@action toggleAdmin(rc *render.RenderContext, props map[string]any) error {
    return db.Exec("UPDATE users SET admin = NOT admin WHERE id = ?", props["id"])
}

func UserCard(user models.User) {
    <div class="card">
        <h3>{user.Name}</h3>
        <button hx-post={c.ActionURL("toggleAdmin")}
                hx-vals={"id": user.ID}>
            Toggle Admin
        </button>
    </div>
}
```

Generated registration:

```go
cr.Define("UserCard").
    Action("toggleAdmin", func(rc *render.RenderContext, props map[string]any) error {
        return db.Exec("UPDATE users SET admin = NOT admin WHERE id = ?", props["id"])
    }).
    Render(func(c *render.ComponentContext) error {
        return RenderUserCard(c, c.Data.(models.User), nil)
    }).
    Register(cr)
```

Mount one handler on the router — `r.Post("/_nano/action/*", reg.HandleAction)` — and every action in every `.gsx` file is live. No router files, no controller wiring, no boilerplate.

### Asset directives (`@css` / `@js`)

Declared at the top of the file, deduplicated into the document head automatically:

```gsx
@css "/static/css/user-card.css"
@js "/static/js/chart-init.js"

func UserCard(user models.User) { ... }
```

The compiler emits `c.RequiresCSS("/static/css/user-card.css")` as the first line of the render function. When the component renders, the asset joins nanite-render's deduplicating graph and `<NANO_ASSETS/>` emits it once into `<head>`. A card rendered 50 times in a loop produces one `<link>` tag.

### Component memoization (`@memo`)

Cache expensive components at the source level — `React.memo` for Go. The directive takes a typed key generator; components whose key repeats skip the render walk and serve cached HTML:

```gsx
@memo(func(rc *render.RenderContext, props UserCardProps) string {
    return props.ID
})

func UserCard(props UserCardProps) {
    // ... expensive HTML generation ...
}
```

The generated registration wraps the component with nanite-render's `Memoize`:

```go
cr.Define("UserCard").
    Render(func(c *render.ComponentContext) error { ... }).
    Register(cr)
cr.Memoize("UserCard", func(rc *render.RenderContext, data any) string {
    props := data.(UserCardProps)   // typed adapter generated automatically
    return props.ID
})
```

Perfect for data-independent components (navbars, static panels) and keyable-by-id components (user cards, product tiles). The keyer returns `""` to skip caching for a particular render.

### Cross-package composition (TSX-style)

Build component libraries in isolated packages and import them — exactly like `import { Button } from "@/components/ui"` in TSX:

```gsx
// views/login.gsx
@import { Button, Input } from "myapp/components/ui"

func LoginForm() {
    <form hx-post="/login">
        <Input inputType="email" name="email" />
        <Button label="Login" />
    </form>
}
```

The compiler resolves `<Button/>` through the **symbol table**: the destructured import maps `Button` → package alias `ui`, and the tag emits a qualified direct call:

```go
// Generated — no reflection, no registry lookup, 0 B/op
if err := ui.RenderButton(c, "Login", nil); err != nil { return err }
```

**The zero-byte marker type.** Go aliases types, not functions — but `type Button = ui.Button` needs `ui.Button` to be a *type*. The compiler emits a marker type alongside every component's render function:

```go
// in the ui package
type Button struct{}  // zero bytes at runtime — exists so the alias compiles
func RenderButton(c *render.ComponentContext, label string, children func(...) error) error { ... }
```

This makes destructured imports work for **both** components and Go types. `@import { User } from "myapp/models"` gives you `User` as a real type for `{user.Name}` expressions; `@import { Button } from "myapp/components/ui"` gives you `<Button/>` resolving to `ui.RenderButton`. One syntax, both worlds.

Same-package composition needs no import — `<UserCard/>` in `dashboard.gsx` resolves to the local `RenderUserCard` automatically.

### Pure gsx layouts (`@yield`)

Layouts are gsx too. `@yield` is where the view body lands:

```gsx
// views/app_layout.gsx
func AppLayout() {
    <html>
        <head><NanoHead/><NanoAssets/></head>
        <body>
            <Navbar/>
            <main>
                @yield
            </main>
        </body>
    </html>
}
```

Compiles to `if err := c.Yield(); err != nil { return err }` — nanite-render's composition hook that writes the pre-rendered view bytes. The two-pass pipeline renders the view first, stashes it on the RenderContext, then the layout's `@yield` writes it in place. No Jade, no `{{ yield }}`, no cross-engine layout composition — one language for the whole page.

### Fragments (`<>...</>`)

Return multiple siblings without a wrapper:

```gsx
func TableColumns(user User) {
    <>
        <td>{user.Name}</td>
        <td>{user.Role}</td>
    </>
}
```

The fragment emits zero bytes — its children compile to sequential `c.WriteString` calls. Same React semantics, same absence of wrapper divs.

### Spread attributes (`{...attrs}`)

Pass a map of attributes to an element dynamically:

```gsx
func Button(label string, attrs map[string]string) {
    <button class="btn" {...attrs}>
        {label}
    </button>
}
```

Compiles to a runtime loop that HTML-escapes each key/value pair and writes `key="value"` into the tag:

```go
for __k, __v := range attrs {
    c.WriteString(" ")
    c.WriteString(html.EscapeString(fmt.Sprint(__k)))
    c.WriteString("=\"")
    c.WriteString(html.EscapeString(fmt.Sprint(__v)))
    c.WriteString("\"")
}
```

Great for HTMX attributes, data-* payloads, and styling maps.

### Native Alpine.js (the golden duo)

HTMX handles network state; Alpine handles ephemeral UI. Both are first-class in `.gsx` — no build step, no JS framework:

```gsx
func Dropdown(isOpen bool, items []string) {
    <div x-data={map[string]any{"open": isOpen, "list": items}}>
        <button @click="open = !open">Toggle</button>

        <ul x-show="open" x-cloak>
            <template x-for="item in list">
                <li x-text="item"></li>
            </template>
        </ul>
    </div>
}
```

**What the compiler does:**

| Alpine syntax | Compiler behavior |
|---|---|
| `@click`, `@keydown`, `@mouseenter` | Pass through untouched — Alpine event directives are never parsed as gsx macros |
| `x-data={goExpr}` / `x-init={goExpr}` | Auto-converts to `c.WriteHydrateProps("x-data", goExpr)` — JSON-serialized, HTML-escaped, valid Alpine state |
| Any `x-` or `@` attribute present | Auto-injects `<style>[x-cloak]{display:none!important}</style>` — no flicker, no manual CSS |
| `hx-post={c.ActionURL("save")}` + `hx-vals="js:{...}"` | HTMX server action triggered from Alpine state — the bridge composes natively |

The generated code:

```go
c.WriteString(`<style>[x-cloak]{display:none!important}</style>`)
c.WriteString(`<button`)
c.WriteString(` @click="open = !open"`)          // Alpine event, raw
c.WriteHydrateProps("x-data", map[string]any{"open": isOpen, "list": items})  // JSON bridge
c.WriteString(`>`)
```

**Two hydration syntaxes, one bridge:**

| Syntax | Use for |
|---|---|
| `@hydrate("x-data", state)` | Explicit — ANY attribute name (Alpine `x-data`, `x-init`, or custom `data-props`) |
| `x-data={goExpr}` | Implicit — the compiler recognizes the Alpine attribute and auto-converts |

Both compile to `c.WriteHydrateProps` — nanite-render's JSON-serialized, HTML-escaped hydration bridge. Use the explicit form when you want control over the attribute name; use the implicit form when you just want Alpine state from a Go value. `@hydrate` remains the general-purpose escape hatch for non-Alpine attributes (`data-*`, HTMX extensions, vanilla JS).

### Flash form errors (`@error`)

The `useActionState` validation pattern without the boilerplate:

```gsx
<form>
    <input type="email" name="email" />
    @error("email")
</form>
```

Expands to the full `GetFormError` check + error span:

```go
if __err := c.Context.GetFormError("email"); __err != "" {
    c.WriteString(`<span class="error">`)
    c.WriteString(html.EscapeString(__err))
    c.WriteString(`</span>`)
}
```

No more hand-writing the same validation markup on every form field.

### Browser live-reload (`gsx watch`)

The Next.js dev experience. `gsx watch` compiles on save, restarts nothing, and **auto-refreshes the browser**:

```bash
GSX_DEV_MODE=1 gsx watch -dir ./views
```

- The compiler injects a tiny SSE script into **layouts** (files with `@yield`) — one connection, no duplication across components
- The reload server runs on the stack's own **nanite router + nanite/sse Hub** at `:3001/reload` — zero-alloc, dogfooded
- Every successful recompile broadcasts `reload`; open browser tabs refresh instantly

Save a `.gsx` file → code regenerates → `go build` → browser reloads. No `CMD+R`, no manual restart.

### Composition from a handler
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

// Asset directives — deduped into <NANO_ASSETS/> in the head.
@css "/static/css/user-card.css"  // → c.RequiresCSS(...)
@js "/static/js/chart-init.js"    // → c.RequiresJS(...)

// Lifecycle decorators — wire into nanite-render's Suspense/OOB.
@oob "card-slot"          // → WithOOB("card-slot")
@async                    // → Async()
@action toggle(user models.User) error {
    // Colocated server action — hoisted to .Action("toggle", fn)
    db.Exec("UPDATE users SET active = NOT active WHERE id = ?", user.ID)
    return nil
}

func UserCard(user User, showEmail bool) {
    // The compiler injects c *render.ComponentContext as the first
    // param. Props are typed — go build catches mismatches.

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

    // Spread attributes — map/struct of key=value pairs
    <button {...attrs}>

    // Fragments — zero-byte structural boundaries
    <>
        <td>a</td>
        <td>b</td>
    </>

    // @yield — write the view body (layouts)
    <main>@yield</main>

    // @hydrate — server state to client (Alpine.js x-data)
    <div @hydrate("x-data", dropdownState)>

    // @error — flash form error span
    <input name="email" />
    @error("email")
}
```

**Case-insensitive lookup:** Component names are normalized — `e.Register("UserCard", fn)` registers under the lowercase key. Templates can write `<UserCard/>` (gsx convention, PascalCase) or `<USERCARD/>` (plain HTML convention, UPPERCASE) — both resolve to the same function. The source of truth is the PascalCase name in the `.gsx` file.

**Three lexer triggers:**

| Trigger | Mode | What happens |
|---|---|---|
| `<` | Tag mode | Uppercase = component call, lowercase = HTML. Non-self-closing collects inner content as children. `{attr}` values become dynamic attributes. |
| `{` | Expression mode | Balanced-brace Go expression, HTML-escaped. In attribute position (`class={expr}`), emitted as split escaped runtime output. |
| `@` | Directive mode | `@if`, `@for`, `@switch`, `@import`, `@children`, `@yield`, `@error`, `@oob`, `@async`, `@fallback`, `@action`, `@css`, `@js` |
| `<div @hydrate(...)>` | Attribute | Server state → client (Alpine.js), compiles to `c.WriteHydrateProps` |

**Imports — compiled to standard Go:**

| .gsx syntax | Generated Go |
|---|---|
| `@import "time"` | `import "time"` |
| `@import models "myapp/models"` | `import models "myapp/models"` |
| `@import { User, Post } from "myapp/models"` | `import models "myapp/models"` + `type User = models.User` — types usable in `{expr}`, component tags resolve to `models.RenderPost` |

**Cross-package resolution:** every component also emits a zero-byte marker type (`type X struct{}`), so destructured symbols compile as Go type aliases for **both** types and components. `<Post/>` in the template resolves to `models.RenderPost(c, ...)` via the symbol table; `Post` as a type in expressions resolves to the alias. No reflection, no registry lookup — direct Go function calls across packages.

---

## Status

Beta. The compiler pipeline is complete and generates valid Go:

- [x] Lexer (3-trigger state machine, block-detection in `scanExpr`)
- [x] Parser (multi-func files, `@import`, decorators, `@if`/`@for`/`@switch`)
- [x] Codegen (`*render.ComponentContext` target, `RegisterX` wrapper)
- [x] `gsx.Engine` implementing `render.Engine`
- [x] Direct component calls (`<Card/>` → `RenderCard(c, props)`)
- [x] `@import` (3 forms)
- [x] Children closures (`<Layout><Card/></Layout>` + `@children`)
- [x] Dynamic attributes (`class={expr}` → `html.EscapeString` at runtime)
- [x] Lifecycle decorators (`@oob`, `@async`, `@fallback` → fluent chain)
- [x] Colocated server actions (`@action` → `.Action()` chain)
- [x] Asset directives (`@css` / `@js` → `c.RequiresCSS`/`c.RequiresJS`)
- [x] Multi-param registration (`BindProps` + generated props struct)
- [x] `@switch` / `@case` / `@default`
- [x] Cross-package component resolution (`@import { X } from "..."` → `pkg.RenderX`)
- [x] Zero-byte marker types (destructured imports for components AND types)
- [x] Conditional imports (no unused fmt/html on static-only components)
- [x] `@yield` layouts (pure gsx — `c.Yield()` composition hook)
- [x] Fragments (`<>...</>` — zero-byte boundaries)
- [x] Spread attributes (`{...attrs}` — runtime escaped `key="value"` loop)
- [x] `@hydrate` (server state → client, Alpine.js bridge)
- [x] Native Alpine.js (`@click` passthrough, `x-data={go}` auto-hydration, `x-cloak` injection)
- [x] `@error` (flash form error macro)
- [x] `gsx compile` CLI
- [x] `gsx watch` with browser live-reload (nanite router + nanite/sse Hub)
- [x] [VS Code extension](https://github.com/xDarkicex/nanite-gsx-vscode) (TextMate grammar, Go + HTML embedding)

---

## License

MIT © 2026 xDarkicex
