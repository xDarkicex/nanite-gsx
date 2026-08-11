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
	Imports       []Import
	FuncName      string
	Params        []string // e.g. ["user models.User"]
	Returns       []string // e.g. ["error"]
	PropsType     string   // e.g. "UserCardProps" if single struct param
	HasComponents bool     // true if body contains <CapitalComponent/>
	Body          ir.NodeStream
}

// Parse reads a .gsx source into a ParsedFile.
func Parse(src []byte) (*ParsedFile, error) {
	s := lexer.NewScanner(src)
	p := &parser{scanner: s, src: src}
	return p.parse()
}

type parser struct {
	scanner *lexer.Scanner
	src     []byte
}

func (p *parser) parse() (*ParsedFile, error) {
	out := &ParsedFile{}

	// Phase 1: preamble — @import directives + func signature.
	for {
		tok := p.scanner.Scan()
		switch tok.Kind {
		case lexer.KindAtImport:
			imp, err := p.parseImport()
			if err != nil {
				return nil, err
			}
			out.Imports = append(out.Imports, imp)
		case lexer.KindFunc:
			if err := p.parseSignature(out); err != nil {
				return nil, err
			}
			// After the func signature, the { opens the body.
			// Fall through to body parsing.
			goto body
		case lexer.KindEOF:
			return nil, fmt.Errorf("unexpected EOF before func signature")
		default:
			// Skip whitespace/comments between directives.
		}
	}

body:
	// Phase 2: template body (HTML with triggers).
	body, err := p.parseBody(out)
	if err != nil {
		return nil, err
	}
	out.Body = body
	return out, nil
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
		p.scanner.Scan() // skip {
		for {
			t := p.scanner.Scan()
			if t.Kind == lexer.KindRBRACE {
				break
			}
			if t.Kind == lexer.KindIdent {
				imp.Symbols = append(imp.Symbols, t.String(p.src))
			}
			// skip commas
		}
		p.scanner.Scan() // skip from
		str := p.scanner.Scan()
		if str.Kind != lexer.KindString {
			return imp, fmt.Errorf("expected string after 'from', got %s", str.String(p.src))
		}
		imp.Path = unquote(str, p.src)
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
		case lexer.KindAtChildren:
			b.AddChildren()

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

func (p *parser) parseIf(b *ir.Builder) error {
	var cond strings.Builder
	depth := 0
	for {
		tok := p.scanner.Scan()
		switch tok.Kind {
		case lexer.KindExpr:
			// { expr } — if the expression is just "{" with whitespace,
			// this is the if-body opener.
			s := tok.String(p.src)
			if s == "{" || (len(s) >= 2 && s[1] == '}') {
				// This { is the body opener; the condition is done.
				b.OpenIf(strings.TrimSpace(cond.String()))
				return nil
			}
			cond.WriteString(s)
		case lexer.KindLBRACE:
			if depth == 0 {
				b.OpenIf(strings.TrimSpace(cond.String()))
				return nil
			}
			cond.WriteString("{")
			depth++
		case lexer.KindRBRACE:
			if depth > 0 {
				cond.WriteString("}")
				depth--
			} else {
				b.CloseControl()
				return nil
			}
		case lexer.KindAtElse:
			b.CloseControl()
			b.OpenElse()
			for {
				t := p.scanner.Scan()
				if t.Kind == lexer.KindLBRACE || t.Kind == lexer.KindExpr {
					return nil
				}
			}
		case lexer.KindEOF:
			return fmt.Errorf("unexpected EOF in @if condition")
		default:
			cond.WriteString(tok.String(p.src))
		}
	}
}

func (p *parser) parseFor(b *ir.Builder) error {
	var cond strings.Builder
	for {
		tok := p.scanner.Scan()
		switch tok.Kind {
		case lexer.KindLBRACE:
			b.OpenFor(strings.TrimSpace(cond.String()))
			return nil
		case lexer.KindExpr:
			b.OpenFor(strings.TrimSpace(cond.String()))
			return nil
		case lexer.KindEOF:
			return fmt.Errorf("unexpected EOF in @for condition")
		default:
			cond.WriteString(tok.String(p.src))
		}
	}
}

func (p *parser) parseSwitch(b *ir.Builder) error {
	var cond strings.Builder
	for {
		tok := p.scanner.Scan()
		switch tok.Kind {
		case lexer.KindLBRACE:
			b.OpenSwitch(strings.TrimSpace(cond.String()))
			return nil
		case lexer.KindExpr:
			b.OpenSwitch(strings.TrimSpace(cond.String()))
			return nil
		default:
			cond.WriteString(tok.String(p.src))
		}
	}
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
