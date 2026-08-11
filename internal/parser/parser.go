// Package parser transforms a .gsx token stream into the
// NodeStream intermediate representation.
package parser

import (
	"fmt"
	"strings"

	"github.com/xDarkicex/nanite-gsx/internal/ir"
	"github.com/xDarkicex/nanite-gsx/internal/lexer"
)

// Import represents a single @import directive.
type Import struct {
	Alias  string   // empty for side-effect, "pkg" for alias, or list for destructured
	Symbols []string // symbols in destructured form @import { A, B } from "..."
	Path   string   // the import path
}

// ParsedFile is the output of the parser — a ready-to-generate
// component definition.
type ParsedFile struct {
	Imports   []Import
	FuncName  string
	Params    []string // e.g. ["user models.User"]
	Returns   []string // e.g. ["error"]
	PropsType string   // e.g. "UserCardProps" if single struct param

	// Decorators (lifecycle bridge to nanite-render).
	OOBID      string // @oob "slot-id" → WithOOB("slot-id")
	Async      bool   // @async → Async()
	FallbackOf string // @fallback(UserProfile) → this func is the fallback
	Fallback   string // resolved: the FuncName of the fallback func

	// Colocated server actions and asset directives.
	Actions    []Action // @action name(rc, props) error { ... }
	CSSAssets  []string // @css "/path"
	JSAssets   []string // @js "/path"

	HasComponents bool // true if body contains <CapitalComponent/>
	Body          ir.NodeStream
}

// Parse reads a .gsx source into one or more ParsedFiles — one
// per func declaration, with decorators attached.
func Parse(src []byte) ([]*ParsedFile, error) {
	s := lexer.NewScanner(src)
	p := &parser{scanner: s, src: src}
	return p.parse()
}

// ParseOne is a convenience for callers that expect a single
// component per file. Returns the first ParsedFile.
func ParseOne(src []byte) (*ParsedFile, error) {
	files, err := Parse(src)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no component found")
	}
	return files[0], nil
}

type parser struct {
	scanner *lexer.Scanner
	src     []byte
}

func (p *parser) parse() ([]*ParsedFile, error) {
	var out []*ParsedFile
	var imports []Import
	var pending Decorators
	var pendingCSS, pendingJS []string
	var pendingActions []Action

	for {
		tok := p.scanner.Scan()
		switch tok.Kind {
		case lexer.KindAtImport:
			imp, err := p.parseImport()
			if err != nil {
				return nil, err
			}
			imports = append(imports, imp)
		case lexer.KindAtOOB, lexer.KindAtAsync, lexer.KindAtFallback:
			dec, err := p.parseDecorator(tok)
			if err != nil {
				return nil, err
			}
			pending = append(pending, dec)
		case lexer.KindAtCSS, lexer.KindAtJS:
			// @css "/path" / @js "/path" — attach to the NEXT
			// func declaration.
			s := p.scanner.Scan()
			if s.Kind != lexer.KindString {
				return nil, fmt.Errorf("expected string after %s", tok.String(p.src))
			}
			path := unquote(s, p.src)
			if tok.Kind == lexer.KindAtCSS {
				pendingCSS = append(pendingCSS, path)
			} else {
				pendingJS = append(pendingJS, path)
			}
		case lexer.KindAtAction:
			act, err := p.parseAction()
			if err != nil {
				return nil, err
			}
			pendingActions = append(pendingActions, act)
		case lexer.KindFunc:
			f := &ParsedFile{
				Imports:   imports,
				CSSAssets: pendingCSS,
				JSAssets:  pendingJS,
				Actions:   pendingActions,
			}
			for _, d := range pending {
				d.apply(f)
			}
			pending = nil
			pendingCSS, pendingJS = nil, nil
			pendingActions = nil
			if err := p.parseSignature(f); err != nil {
				return nil, err
			}
			body, err := p.parseBody(f)
			if err != nil {
				return nil, err
			}
			f.Body = body
			out = append(out, f)
		case lexer.KindEOF:
			if len(out) == 0 {
				return nil, fmt.Errorf("unexpected EOF before func signature")
			}
			resolveFallbacks(out)
			return out, nil
		default:
			// Skip whitespace/comments between declarations.
		}
	}
}

// resolveFallbacks links each func marked @fallback(X) to X's
// Fallback field, so the codegen can emit the Fallback closure.
func resolveFallbacks(files []*ParsedFile) {
	byName := make(map[string]*ParsedFile, len(files))
	for _, f := range files {
		byName[f.FuncName] = f
	}
	for _, f := range files {
		if f.FallbackOf != "" {
			if target, ok := byName[f.FallbackOf]; ok {
				target.Fallback = f.FuncName
			}
		}
	}
}

// Action is a colocated server action: Go mutation logic that
// lives next to the component. Hoisted into the fluent builder's
// .Action(name, fn) at registration.
type Action struct {
	Name string // "toggleAdmin"
	Sig  string // the params between parens, verbatim
	Body string // the raw Go body between braces
}

// Decorators are @-directives that precede a func declaration
// and wire the component into nanite-render lifecycles.
type Decorators []Decorator

// Decorator is a single lifecycle directive.
type Decorator struct {
	Kind  string // "oob", "async", "fallback"
	Value string // slot id for oob, component name for fallback
}

func (d Decorator) apply(f *ParsedFile) {
	switch d.Kind {
	case "oob":
		f.OOBID = d.Value
	case "async":
		f.Async = true
	case "fallback":
		f.FallbackOf = d.Value
	}
}

// parseAction reads @action name(sig) { body }. The signature
// and body are captured as raw Go text (verbatim).
func (p *parser) parseAction() (Action, error) {
	var act Action

	// name
	tok := p.scanner.Scan()
	if tok.Kind != lexer.KindIdent {
		return act, fmt.Errorf("expected action name after @action")
	}
	act.Name = tok.String(p.src)

	// signature: read raw bytes from ( to the matching )
	depth := 0
	var sig strings.Builder
	for {
		b := p.scanner.NextByte()
		if b == 0 {
			return act, fmt.Errorf("unexpected EOF in @action signature")
		}
		switch b {
		case '(':
			depth++
			if depth > 1 {
				sig.WriteByte(b)
			}
		case ')':
			depth--
			if depth == 0 {
				goto sigDone
			}
			sig.WriteByte(b)
		default:
			sig.WriteByte(b)
		}
	}
sigDone:
	act.Sig = strings.TrimSpace(sig.String())

	// Skip return type (error) and whitespace to the {.
	for {
		b := p.scanner.NextByte()
		if b == 0 {
			return act, fmt.Errorf("unexpected EOF before @action body")
		}
		if b == '{' {
			break
		}
	}

	// Body: brace-counted raw Go.
	depth = 1
	var body strings.Builder
	for depth > 0 {
		b := p.scanner.NextByte()
		if b == 0 {
			return act, fmt.Errorf("unexpected EOF in @action body")
		}
		switch b {
		case '{':
			depth++
			body.WriteByte(b)
		case '}':
			depth--
			if depth == 0 {
				goto bodyDone
			}
			body.WriteByte(b)
		case '"', '\'', '`':
			body.WriteByte(b)
			// Skip the string contents.
			for {
				c := p.scanner.NextByte()
				if c == 0 {
					return act, fmt.Errorf("unexpected EOF in @action string")
				}
				body.WriteByte(c)
				if c == '\\' {
					e := p.scanner.NextByte()
					if e == 0 {
						return act, fmt.Errorf("unexpected EOF in @action string escape")
					}
					body.WriteByte(e)
					continue
				}
				if c == b {
					break
				}
			}
		default:
			body.WriteByte(b)
		}
	}
bodyDone:
	act.Body = body.String()
	return act, nil
}

func (p *parser) parseDecorator(tok lexer.Token) (Decorator, error) {
	switch tok.Kind {
	case lexer.KindAtOOB:
		// @oob "slot-id"
		s := p.scanner.Scan()
		if s.Kind != lexer.KindString {
			return Decorator{}, fmt.Errorf("expected string after @oob, got %s", s.String(p.src))
		}
		return Decorator{Kind: "oob", Value: unquote(s, p.src)}, nil
	case lexer.KindAtAsync:
		return Decorator{Kind: "async"}, nil
	case lexer.KindAtFallback:
		// @fallback(ComponentName)
		s := p.scanner.Scan()
		if s.Kind != lexer.KindLParen {
			return Decorator{}, fmt.Errorf("expected ( after @fallback")
		}
		name := p.scanner.Scan()
		if name.Kind != lexer.KindIdent {
			return Decorator{}, fmt.Errorf("expected component name in @fallback")
		}
		close := p.scanner.Scan()
		if close.Kind != lexer.KindRParen {
			return Decorator{}, fmt.Errorf("expected ) after @fallback name")
		}
		return Decorator{Kind: "fallback", Value: name.String(p.src)}, nil
	}
	return Decorator{}, fmt.Errorf("unknown decorator")
}

func (p *parser) parseImport() (Import, error) {
	imp := Import{}
	// After @import, options:
	//   1. "path"           → side-effect import
	//   2. alias "path"     → aliased import
	//   3. { A, B } from "path" → destructured import
	//   4. alias from "path"     → aliased import with from keyword

	tok := p.scanner.Scan()
	switch tok.Kind {
	case lexer.KindString:
		// @import "path"
		imp.Path = unquote(tok, p.src)
		return imp, nil
	case lexer.KindLBRACE:
		// @import { A, B } from "path"
		// The { was the token we matched — scan the symbols
		// inside until the closing brace.
		for {
			t := p.scanner.Scan()
			switch t.Kind {
			case lexer.KindRBRACE:
				goto from
			case lexer.KindIdent:
				imp.Symbols = append(imp.Symbols, t.String(p.src))
			case lexer.KindComma:
				// separator — skip
			case lexer.KindEOF:
				return imp, fmt.Errorf("unexpected EOF in @import { ... }")
			}
		}
	from:
		// Optional "from" keyword, then the path string.
		f := p.scanner.Scan()
		if f.Kind == lexer.KindIdent && f.String(p.src) == "from" {
			f = p.scanner.Scan()
		}
		if f.Kind != lexer.KindString {
			return imp, fmt.Errorf("expected string after 'from', got %s", f.String(p.src))
		}
		imp.Path = unquote(f, p.src)
		return imp, nil
	case lexer.KindIdent:
		// Could be "alias from "path"" or just "alias"
		alias := tok.String(p.src)
		next := p.scanner.Scan()
		if next.Kind == lexer.KindIdent && next.String(p.src) == "from" {
			// alias from "path"
			str := p.scanner.Scan()
			if str.Kind != lexer.KindString {
				return imp, fmt.Errorf("expected string after 'from'")
			}
			imp.Alias = alias
			imp.Path = unquote(str, p.src)
			return imp, nil
		}
		// Just alias — next should be a string path
		if next.Kind == lexer.KindString {
			imp.Alias = alias
			imp.Path = unquote(next, p.src)
			return imp, nil
		}
		return imp, fmt.Errorf("unexpected token after @import ident: %s", next.String(p.src))
	}
	return imp, fmt.Errorf("unexpected token after @import: %s", tok.String(p.src))
}

func (p *parser) parseSignature(out *ParsedFile) error {
	// Read the function name.
	tok := p.scanner.Scan()
	if tok.Kind != lexer.KindIdent {
		return fmt.Errorf("expected function name, got %s", tok.String(p.src))
	}
	out.FuncName = tok.String(p.src)

	// Read everything from here to { using NextByte. The scanner
	// is positioned right after the function name.
	var sigBuf strings.Builder
	for {
		b := p.scanner.NextByte()
		if b == 0 {
			return fmt.Errorf("unexpected EOF in function signature")
		}
		if b == '{' {
			// The function body opens. Don't enter template mode
			// yet — we need to consume this { as a plain token
			// first so the scanner doesn't scan it as an expr.
			raw := strings.TrimSpace(sigBuf.String())
			// Strip leading ( and trailing ).
			raw = strings.TrimPrefix(raw, "(")
			raw = strings.TrimSuffix(raw, ")")
			parts := splitTopLevel(raw, ',')
			for i := range parts {
				parts[i] = strings.TrimSpace(parts[i])
				if len(parts[i]) > 0 && parts[i][0] == '(' {
					out.Returns = []string{strings.Trim(parts[i], "() ")}
					parts = parts[:i]
					break
				}
			}
			if len(parts) > 0 && parts[0] != "" {
				out.Params = parts
			}
			// NOW enter template mode — the scanner position is
			// right after { and will start seeing <, {, @ triggers.
			p.scanner.EnterTemplate()
			return nil
		}
		sigBuf.WriteByte(b)
	}
}

func splitTopLevel(s string, sep byte) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(', '<':
			depth++
		case ')', '>':
			depth--
		case sep:
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func (p *parser) parseBody(out *ParsedFile) (ir.NodeStream, error) {
	b := ir.NewBuilder()
	b.SetView(out.FuncName, out.Params, out.Returns)

	blockDepth := 1 // the function's own { counts as depth 1
	for {
		tok := p.scanner.Scan()
		switch tok.Kind {
		case lexer.KindEOF:
			return b.Stream(), nil
		case lexer.KindText, lexer.KindIdent:
			// In template mode, bare identifiers (not inside {})
			// are just text content — like "ADMIN" between tags.
			b.AddText(tok.String(p.src))
		case lexer.KindExpr:
			s := tok.String(p.src)
			expr := s[1 : len(s)-1]
			b.AddExpr(expr)
		case lexer.KindLT:
			if err := p.parseTag(b, tok); err != nil {
				return ir.NodeStream{}, err
			}
		case lexer.KindAtIf:
			blockDepth++
			if err := p.parseIf(b); err != nil {
				return ir.NodeStream{}, err
			}
		case lexer.KindAtElse:
			// Close the previous @if and open an @else block.
			b.CloseControl()
			b.OpenElse()
			blockDepth++
			// Consume the block-opening { (no condition on else).
			if _, err := p.readCond(); err != nil {
				return ir.NodeStream{}, err
			}
		case lexer.KindAtChildren:
			b.AddChildren()
		case lexer.KindAtYield:
			b.AddYield()

		case lexer.KindAtFor:
			blockDepth++
			if err := p.parseFor(b); err != nil {
				return ir.NodeStream{}, err
			}
		case lexer.KindAtSwitch:
			blockDepth++
			if err := p.parseSwitch(b); err != nil {
				return ir.NodeStream{}, err
			}
		case lexer.KindAtCase:
			// @case "admin": — value until the colon.
			var val strings.Builder
			for {
				t := p.scanner.Scan()
				if t.Kind == lexer.KindColon {
					break
				}
				if t.Kind == lexer.KindEOF {
					return ir.NodeStream{}, fmt.Errorf("unexpected EOF in @case")
				}
				val.WriteString(t.String(p.src))
			}
			b.OpenCase(strings.TrimSpace(val.String()))
		case lexer.KindAtDefault:
			// @default:
			p.scanner.Scan() // consume the colon
			b.OpenDefault()
		case lexer.KindRBRACE:
			blockDepth--
			if blockDepth <= 0 {
				return b.Stream(), nil
			}
			b.CloseControl()
		case lexer.KindError:
			return ir.NodeStream{}, fmt.Errorf("lexer error: %s", tok.String(p.src))
		}
	}
}

func (p *parser) parseTag(b *ir.Builder, openTok lexer.Token) error {
	s := openTok.String(p.src)
	// Strip < and >
	inner := s[1 : len(s)-1]

	// Fragments: <> and </>
	if inner == "" {
		b.OpenFragment()
		return nil
	}
	if inner == "/" {
		b.CloseFragment()
		return nil
	}

	// Self-closing: <Tag /> or <Tag/>
	if strings.HasSuffix(inner, "/") {
		inner = strings.TrimSuffix(inner, "/")
		inner = strings.TrimSpace(inner)
		name, attrs := parseTagName(inner)
		if isCapital(name) {
			b.AddComponent(name, attrs...)
		} else {
			b.OpenTag(name, attrs...)
			b.CloseTag(name)
		}
		return nil
	}

	// Closing component tag: </Name>
	if strings.HasPrefix(inner, "/") {
		name := strings.TrimSpace(inner[1:])
		if isCapital(name) {
			b.CloseComponent(name)
		} else {
			b.CloseTag(name)
		}
		return nil
	}

	// Opening tag: <Tag attrs>
	name, attrs := parseTagName(inner)
	if isCapital(name) {
		b.OpenComponent(name, attrs...)
	} else {
		b.OpenTag(name, attrs...)
	}
	return nil
}

// readCond captures the raw source bytes from the current
// position up to the block-opening { — preserving the Go exactly
// (spaces, :=, operators). String literals in the condition are
// skipped so braces inside strings don't confuse the search.
func (p *parser) readCond() (string, error) {
	var cond strings.Builder
	for {
		b := p.scanner.NextByte()
		if b == 0 {
			return "", fmt.Errorf("unexpected EOF in control-flow condition")
		}
		switch b {
		case '{':
			return strings.TrimSpace(cond.String()), nil
		case '"', '\'', '`':
			cond.WriteByte(b)
			for {
				c := p.scanner.NextByte()
				if c == 0 {
					return "", fmt.Errorf("unexpected EOF in condition string")
				}
				cond.WriteByte(c)
				if c == '\\' {
					e := p.scanner.NextByte()
					if e == 0 {
						return "", fmt.Errorf("unexpected EOF in condition string escape")
					}
					cond.WriteByte(e)
					continue
				}
				if c == b {
					break
				}
			}
		default:
			cond.WriteByte(b)
		}
	}
}

func (p *parser) parseIf(b *ir.Builder) error {
	cond, err := p.readCond()
	if err != nil {
		return err
	}
	b.OpenIf(cond)
	return nil
}

func (p *parser) parseFor(b *ir.Builder) error {
	cond, err := p.readCond()
	if err != nil {
		return err
	}
	b.OpenFor(cond)
	return nil
}

func (p *parser) parseSwitch(b *ir.Builder) error {
	cond, err := p.readCond()
	if err != nil {
		return err
	}
	b.OpenSwitch(cond)
	return nil
}

// parseTagName splits "<Tagname key=val key2=val2" into name and
// alternating key-value pairs.
func parseTagName(s string) (name string, attrs []string) {
	s = strings.TrimSpace(s)
	// Find the first whitespace or end.
	i := 0
	for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '\n' {
		i++
	}
	name = s[:i]
	rest := strings.TrimSpace(s[i:])
	if rest == "" {
		return name, nil
	}
	return name, parseAttrs(rest)
}

func isCapital(s string) bool {
	if len(s) == 0 {
		return false
	}
	return s[0] >= 'A' && s[0] <= 'Z'
}

func parseAttrs(s string) []string {
	var attrs []string
	for len(s) > 0 {
		// Skip whitespace.
		s = strings.TrimLeft(s, " \t\n")
		if len(s) == 0 {
			break
		}
		// Bare spread at attribute position: {...attrs}
		if s[0] == '{' {
			depth := 1
			j := 1
			for j < len(s) && depth > 0 {
				if s[j] == '{' {
					depth++
				} else if s[j] == '}' {
					depth--
				}
				if depth == 0 {
					j++
					break
				}
				j++
			}
			expr := s[1 : j-1]
			if strings.HasPrefix(expr, "...") {
				attrs = append(attrs, "...", expr[3:])
			} else {
				// Bare expression attribute (React-style bool).
				attrs = append(attrs, expr, "true")
			}
			s = s[j:]
			continue
		}
		// Read key.
		i := 0
		for i < len(s) && s[i] != '=' && s[i] != ' ' && s[i] != '\t' {
			i++
		}
		key := s[:i]
		s = s[i:]
		if len(s) == 0 || s[0] != '=' {
			// Boolean attribute.
			attrs = append(attrs, key, "")
			continue
		}
		s = s[1:] // skip =
		// Read value.
		if len(s) == 0 {
			attrs = append(attrs, key, "")
			break
		}
		if s[0] == '"' || s[0] == '\'' {
			q := s[0]
			s = s[1:]
			j := 0
			for j < len(s) && s[j] != q {
				if s[j] == '\\' && j+1 < len(s) {
					j += 2
				} else {
					j++
				}
			}
			val := s[:j]
			if j < len(s) {
				s = s[j+1:]
			} else {
				s = ""
			}
			attrs = append(attrs, key, val)
		} else if s[0] == '{' {
			// Spread attribute: {...attrs} — a map/struct of
			// key/value pairs written at runtime.
			if len(s) >= 4 && s[1] == '.' && s[2] == '.' && s[3] == '.' {
				depth := 1
				j := 4
				for j < len(s) && depth > 0 {
					if s[j] == '{' {
						depth++
					} else if s[j] == '}' {
						depth--
					}
					if depth == 0 {
						j++
						break
					}
					j++
				}
				attrs = append(attrs, "...", s[4:j-1])
				s = s[j:]
				continue
			}
			// Expression attribute: class={expr}
			depth := 1
			j := 1
			for j < len(s) && depth > 0 {
				if s[j] == '{' {
					depth++
				} else if s[j] == '}' {
					depth--
				}
				if depth == 0 {
					j++
					break
				}
				j++
			}
			attrs = append(attrs, key, s[1:j-1])
			attrs = append(attrs, "_dynamic") // marker for the builder
			s = s[j:]
		} else {
			// Unquoted value — read until whitespace.
			j := 0
			for j < len(s) && s[j] != ' ' && s[j] != '\t' && s[j] != '>' {
				j++
			}
			attrs = append(attrs, key, s[:j])
			s = s[j:]
		}
	}
	return attrs
}

func unquote(tok lexer.Token, src []byte) string {
	s := tok.String(src)
	if len(s) >= 2 {
		q := s[0]
		if (q == '"' || q == '\'' || q == '`') && s[len(s)-1] == q {
			return s[1 : len(s)-1]
		}
	}
	return s
}
