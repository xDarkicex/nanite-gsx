package ir

// Builder constructs a NodeStream programmatically. Used both by
// the parser (for automatic IR construction) and by tests /
// prototypes (for hand-wired IR).
type Builder struct {
	stream NodeStream
	stack  []int // parent node indices during tree construction
}

// NewBuilder returns a Builder with a synthetic document root
// (KindFragment) at index 0. The root is pushed onto the stack so
// every top-level node gets proper parent/sibling links — the
// codegen relies on sibling chains for @else, @case, etc.
func NewBuilder() *Builder {
	b := &Builder{}
	b.stream.Kind = append(b.stream.Kind, KindFragment)
	b.ensureLen(1)
	b.stack = append(b.stack, 0)
	return b
}

// root returns true if i is the synthetic document root.
func root(i int) bool { return i == 0 }

// Stream returns the constructed NodeStream. The builder remains
// usable after this call (the stream is NOT consumed).
func (b *Builder) Stream() NodeStream {
	s := b.stream
	s.Count = len(s.Kind)
	return s
}

// SetView configures the generated function signature.
func (b *Builder) SetView(name string, props, returns []string) {
	b.stream.ViewName = name
	b.stream.ViewProps = props
	b.stream.ViewReturns = returns
}

// AddText appends a text node containing raw HTML text (or Go
// preamble text before the first @view directive).
func (b *Builder) AddText(text string) {
	b.stream.Kind = append(b.stream.Kind, KindText)
	b.stream.Text = append(b.stream.Text, text)
	b.addCommon()
}

// AddExpr appends an expression node ({ expr }) whose value is
// HTML-escaped in the output.
func (b *Builder) AddExpr(goCode string) {
	b.stream.Kind = append(b.stream.Kind, KindExpr)
	b.stream.Text = append(b.stream.Text, goCode)
	b.addCommon()
}

// AddRawExpr appends a raw expression node ({! expr }) — no
// HTML escaping in the output (for attribute values, json).
func (b *Builder) AddRawExpr(goCode string) {
	b.stream.Kind = append(b.stream.Kind, KindRawExpr)
	b.stream.Text = append(b.stream.Text, goCode)
	b.addCommon()
}

// AddComponent appends a self-closing component call node
// (<Name props/>). attrs is alternating key, value pairs.
func (b *Builder) AddComponent(name string, attrs ...string) {
	b.stream.Kind = append(b.stream.Kind, KindComponent)
	b.stream.Tag = append(b.stream.Tag, name)
	b.setAttrs(attrs...)
	b.addCommon()
}

// OpenComponent starts a non-self-closing component tag
// (<Name props>). Subsequent nodes become children until
// CloseComponent is called.
func (b *Builder) OpenComponent(name string, attrs ...string) {
	idx := len(b.stream.Kind)
	b.stream.Kind = append(b.stream.Kind, KindComponent)
	b.stream.Tag = append(b.stream.Tag, name)
	b.setAttrs(attrs...)
	b.addCommonRaw(idx)
	b.stack = append(b.stack, idx)
}

// AddChildren appends a @children node — where the children
// closure is invoked.
func (b *Builder) AddChildren() {
	b.stream.Kind = append(b.stream.Kind, KindChildren)
	b.addCommon()
}

// AddYield appends a @yield node — where the pre-rendered view
// body is written (layout composition).
func (b *Builder) AddYield() {
	b.stream.Kind = append(b.stream.Kind, KindYield)
	b.addCommon()
}

// AddError appends an @error("field") node — the form error span
// boilerplate.
func (b *Builder) AddError(field string) {
	b.stream.Kind = append(b.stream.Kind, KindError)
	b.stream.Text = append(b.stream.Text, field)
	b.addCommon()
}

// OpenFragment starts a <></> fragment — a no-op structural
// boundary whose children emit consecutively.
func (b *Builder) OpenFragment() {
	idx := len(b.stream.Kind)
	b.stream.Kind = append(b.stream.Kind, KindFragment)
	b.addCommonRaw(idx)
	b.stack = append(b.stack, idx)
}

// CloseFragment closes the innermost fragment.
func (b *Builder) CloseFragment() {
	if len(b.stack) > 1 {
		b.stack = b.stack[:len(b.stack)-1]
	}
}

// CloseComponent closes the innermost open component tag
// (</Name>). Children between OpenComponent and CloseComponent
// are the component's body.
func (b *Builder) CloseComponent(name string) {
	if len(b.stack) > 0 {
		if len(b.stack) > 1 {
			b.stack = b.stack[:len(b.stack)-1]
		}
	}
}

// OpenTag appends an opening HTML tag and pushes it onto the
// parent stack so subsequent nodes become its children.
func (b *Builder) OpenTag(tag string, attrs ...string) {
	idx := len(b.stream.Kind)
	b.stream.Kind = append(b.stream.Kind, KindHTMLTag)
	b.stream.Tag = append(b.stream.Tag, tag)
	b.setAttrs(attrs...)
	b.addCommonRaw(idx)
	b.stack = append(b.stack, idx)
}

// CloseTag closes the innermost open tag.
func (b *Builder) CloseTag(tag string) {
	b.stream.Kind = append(b.stream.Kind, KindHTMLClose)
	b.stream.Tag = append(b.stream.Tag, tag)
	b.addCommon()
	if len(b.stack) > 0 {
		if len(b.stack) > 1 {
			b.stack = b.stack[:len(b.stack)-1]
		}
	}
}

// OpenIf starts an @if block.
func (b *Builder) OpenIf(cond string) {
	idx := len(b.stream.Kind)
	b.stream.Kind = append(b.stream.Kind, KindIf)
	b.stream.Cond = append(b.stream.Cond, cond)
	b.addCommonRaw(idx)
	b.stack = append(b.stack, idx)
}

// OpenElse starts an @else block (must follow an OpenIf block).
func (b *Builder) OpenElse() {
	idx := len(b.stream.Kind)
	b.stream.Kind = append(b.stream.Kind, KindElse)
	b.addCommonRaw(idx)
	b.stack = append(b.stack, idx)
}

// OpenSwitch starts an @switch block.
func (b *Builder) OpenSwitch(cond string) {
	idx := len(b.stream.Kind)
	b.stream.Kind = append(b.stream.Kind, KindSwitch)
	b.stream.Cond = append(b.stream.Cond, cond)
	b.addCommonRaw(idx)
	b.stack = append(b.stack, idx)
}

// OpenCase starts a @case block inside a @switch.
func (b *Builder) OpenCase(cond string) {
	idx := len(b.stream.Kind)
	b.stream.Kind = append(b.stream.Kind, KindCase)
	b.stream.Cond = append(b.stream.Cond, cond)
	b.addCommonRaw(idx)
	b.stack = append(b.stack, idx)
}

// OpenDefault starts a @default block inside a @switch.
func (b *Builder) OpenDefault() {
	idx := len(b.stream.Kind)
	b.stream.Kind = append(b.stream.Kind, KindDefault)
	b.addCommonRaw(idx)
	b.stack = append(b.stack, idx)
}

// CloseControl closes the innermost control flow block (@if /
// @else / @for / @switch). Like CloseTag, the close is a no-op
// node — it doesn't exist in the stream — so we just pop.
func (b *Builder) CloseControl() {
	if len(b.stack) > 0 {
		if len(b.stack) > 1 {
			b.stack = b.stack[:len(b.stack)-1]
		}
	}
}

// linkChild sets parent[idx] and appends idx to the parent's
// sibling chain. The parent must already exist in the stream.
func (b *Builder) linkChild(parent, idx int) {
	b.stream.Parent[idx] = int32(parent)
	prevFC := int(b.stream.FirstChild[parent])
	if prevFC == -1 {
		b.stream.FirstChild[parent] = int32(idx)
	} else {
		cur := prevFC
		for b.stream.NextSibling[cur] != -1 {
			cur = int(b.stream.NextSibling[cur])
		}
		b.stream.NextSibling[cur] = int32(idx)
	}
}

// OpenFor starts an @for block.
func (b *Builder) OpenFor(cond string) {
	idx := len(b.stream.Kind)
	b.stream.Kind = append(b.stream.Kind, KindFor)
	b.stream.Cond = append(b.stream.Cond, cond)
	b.addCommonRaw(idx)
	b.stack = append(b.stack, idx)
}

func (b *Builder) setAttrs(attrs ...string) {
	start := uint32(len(b.stream.AttrKeys))
	for i := 0; i < len(attrs); {
		key := attrs[i]
		i++
		if i >= len(attrs) {
			break
		}
		val := attrs[i]
		i++
		dynamic := false
		// Check for _dynamic marker after the value.
		if i < len(attrs) && attrs[i] == "_dynamic" {
			dynamic = true
			i++
		}
		b.stream.AttrKeys = append(b.stream.AttrKeys, key)
		b.stream.AttrVals = append(b.stream.AttrVals, val)
		b.stream.AttrDynamic = append(b.stream.AttrDynamic, dynamic)
		b.stream.AttrSpread = append(b.stream.AttrSpread, key == "...")
		b.stream.AttrHydrate = append(b.stream.AttrHydrate, key == "@hydrate")
	}
	b.stream.AttrStart = append(b.stream.AttrStart, start)
	b.stream.AttrEnd = append(b.stream.AttrEnd, uint32(len(b.stream.AttrKeys)))
}

// setDynamicAttr adds an attribute whose value is a Go
// expression — the codegen emits html.EscapeString(fmt.Sprint(val))
// instead of a static string.
func (b *Builder) setDynamicAttr(key, expr string) {
	b.stream.AttrKeys = append(b.stream.AttrKeys, key)
	b.stream.AttrVals = append(b.stream.AttrVals, expr)
	b.stream.AttrDynamic = append(b.stream.AttrDynamic, true)
	b.stream.AttrSpread = append(b.stream.AttrSpread, false)
	b.stream.AttrHydrate = append(b.stream.AttrHydrate, false)
}

func (b *Builder) addCommon() {
	idx := len(b.stream.Kind) - 1
	b.addCommonRaw(idx)
}

func (b *Builder) addCommonRaw(idx int) {
	b.ensureLen(idx + 1)
	if len(b.stack) > 0 {
		b.linkChild(b.stack[len(b.stack)-1], idx)
	}
}

func (b *Builder) ensureLen(n int) {
	for len(b.stream.Kind) < n {
		b.stream.Kind = append(b.stream.Kind, 0)
	}
	for len(b.stream.Flags) < n {
		b.stream.Flags = append(b.stream.Flags, 0)
	}
	for len(b.stream.Text) < n {
		b.stream.Text = append(b.stream.Text, "")
	}
	for len(b.stream.Tag) < n {
		b.stream.Tag = append(b.stream.Tag, "")
	}
	for len(b.stream.AttrStart) < n {
		b.stream.AttrStart = append(b.stream.AttrStart, 0)
	}
	for len(b.stream.AttrEnd) < n {
		b.stream.AttrEnd = append(b.stream.AttrEnd, 0)
	}
	for len(b.stream.Cond) < n {
		b.stream.Cond = append(b.stream.Cond, "")
	}
	for len(b.stream.Parent) < n {
		b.stream.Parent = append(b.stream.Parent, -1)
	}
	for len(b.stream.FirstChild) < n {
		b.stream.FirstChild = append(b.stream.FirstChild, -1)
	}
	for len(b.stream.NextSibling) < n {
		b.stream.NextSibling = append(b.stream.NextSibling, -1)
	}
}

func (b *Builder) findFirstChild(parent int) int {
	for i, p := range b.stream.Parent {
		if int(p) == parent {
			return i
		}
	}
	return -1
}
