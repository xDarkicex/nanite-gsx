package gsx

import (
	"strings"

	"github.com/xDarkicex/nanite-render"
)

// Engine implements render.Engine — the adapter that plugs
// AOT-compiled .gsx views into nanite-render's universal
// composition hub. It works exactly like engine.Jade,
// engine.HTMLTemplate, and engine.HTML: register views by name,
// render them through the same RenderNamed / RenderPage API.
//
// Because .gsx views are AOT-compiled to Go functions, the
// Compile method ignores source bytes and does a direct map
// lookup — there's no runtime parsing. The view function is
// stored on the Program's EngineData field; Execute type-asserts
// and calls it.
//
// Dual-path execution:
//
//  1. Internal fast path — when a .gsx component calls another
//     .gsx component (<UserCard user={u} />), the generated code
//     calls RenderUserCard(c, u) directly. Zero allocations, zero
//     registry lookups, 100% type-checked by go build.
//
//  2. External dynamic path — when a router, a jade layout, or
//     render.RenderNamed dispatches by string name, it goes
//     through Engine.Execute -> EngineData -> function call. This
//     is the path for cross-engine composition.
type Engine struct {
	views map[string]func(c *render.ComponentContext, data any) error
}

// New returns an initialized Engine.
func New() *Engine {
	return &Engine{views: make(map[string]func(c *render.ComponentContext, data any) error)}
}

// Register adds an AOT-compiled view function under name.
// Called from the generated RegisterX(e *Engine) functions.
// Name is stored lowercased so lookups are case-insensitive —
// a .gsx component registered as "UserCard" is reachable as
// "UserCard" (gsx convention), "USERCARD" (plain HTML
// convention), or any case variant.
func (e *Engine) Register(name string, fn func(c *render.ComponentContext, data any) error) {
	e.views[strings.ToLower(name)] = fn
}

// Name implements render.Engine.
func (e *Engine) Name() string { return "gsx" }

// Compile implements render.Engine. Source bytes are ignored —
// the view is already compiled. Returns a Program with the view
// function stored in EngineData.
func (e *Engine) Compile(_ []byte, name string) (*render.Program, error) {
	fn, ok := e.views[strings.ToLower(name)]
	if !ok {
		return nil, render.ErrTemplateNotFound
	}
	return &render.Program{
		Engine:     "gsx",
		Name:       name,
		EngineData: fn,
	}, nil
}

// Execute implements render.Engine. The Program's EngineData
// must be a view function registered via Register.
func (e *Engine) Execute(p *render.Program, w render.ByteWriter, rc *render.RenderContext, data any) error {
	if p == nil || p.EngineData == nil {
		return nil
	}
	fn, ok := p.EngineData.(func(c *render.ComponentContext, data any) error)
	if !ok {
		return render.ErrEngineNotFound
	}
	return fn(&render.ComponentContext{Writer: w, Context: rc, Data: data}, data)
}
