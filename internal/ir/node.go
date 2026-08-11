// Package ir defines the NodeStream intermediate representation
// for parsed .gsx files — a flat structure-of-arrays layout,
// similar in spirit to nanite-render's SoA, but for gsx source
// rather than HTML. The code generator walks this stream to
// emit Go source code.
package ir

// NodeKind classifies a node in the stream.
type NodeKind uint8

const (
	KindText      NodeKind = iota // static HTML text (or Go preamble text)
	KindExpr                     // { expr } — Go expression, HTML-escaped
	KindRawExpr                  // {! expr } — raw Go expression (no escaping)
	KindComponent                // <CapitalName props />
	KindHTMLTag                  // <lowercase attrs> — open tag
	KindHTMLClose                // </lowercase> — close tag
	KindIf                       // @if cond { ... } — control flow
	KindFor                      // @for init; cond; post { ... }
	KindChildren                 // @children — render children closure
	KindSwitch                   // @switch expr { ... }
	KindElse                     // @else { ... } (sibling of KindIf)
	KindCase                     // @case val: (child of KindSwitch)
	KindDefault                  // @default: (child of KindSwitch)
	KindView                     // marker: start of a @view/component region
	KindFragment                 // synthetic document root (index 0) or <></> fragment
	KindYield                    // @yield — write the pre-rendered view body
	KindError                    // @error("field") — form error span
)

// NodeStream is the flat, structure-of-arrays AST produced by
// the parser. Each NodeKind has a specific subset of fields that
// are populated; the code generator switches on Kind to decide
// what to emit.
//
// Count is the number of nodes; indices 0..Count-1 are valid.
// Sibling links (FirstChild/NextSibling) form a tree, so the
// code generator can walk depth-first with no recursion stack.
type NodeStream struct {
	Kind  []NodeKind
	Flags []uint8 // reserved for future use (self-closing, void, etc.)

	// Text payloads (KindText contains the HTML fragment;
	// KindExpr/RawExpr contains the Go expression string).
	Text []string

	// Tag payloads (KindComponent's name, KindHTMLTag/KindHTMLClose
	// element name).
	Tag []string

	// Attributes for KindComponent and KindHTMLTag. Indexed by
	// (AttrStart[i], AttrEnd[i]); each attribute is a key="value"
	// pair stored as consecutive entries in AttrKeys/AttrVals.
	AttrStart   []uint32
	AttrEnd     []uint32
	AttrKeys    []string
	AttrVals    []string  // always strings in the IR; codegen decides escaping
	AttrDynamic []bool    // true if AttrVals[i] is a Go expression {expr}
	AttrSpread  []bool    // true if AttrKeys[i]=="..." — spread a map/struct of attrs
	AttrHydrate []bool    // true if AttrKeys[i]=="@hydrate" — value is "attrName\x00expr"

	// Control flow condition (KindIf, KindFor, KindSwitch).
	Cond []string

	// Go preamble (the imports + types + @view signature before
	// the first @view directive). Stored as the first node in the
	// stream (index 0, KindText, with the full preamble).
	// View name and props type for the generated function
	// signature.
	ViewName    string   // e.g. "UserProfile"
	ViewProps   []string // e.g. ["props ProfileProps"] (param names + types)
	ViewReturns []string // e.g. ["error"] or empty

	// Tree links: parent/child/sibling navigation.
	Parent      []int32
	FirstChild  []int32
	NextSibling []int32

	Count int
}
