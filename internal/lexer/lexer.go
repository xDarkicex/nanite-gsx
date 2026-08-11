// Package lexer scans .gsx source into a token stream.
// SWAR-driven (see swar.go) — 8 bytes per cycle for the hot-path
// transitions (<, {, @, ", ').
package lexer

// Kind classifies a .gsx token.
type Kind uint8

const (
	KindEOF   Kind = iota // end of source
	KindError             // malformed input

	// Preamble: @import directives and func signature.
	KindAtImport  // @import ... (handled as a directive)
	KindFrom      // the "from" keyword in @import
	KindFunc      // func keyword
	KindIdent     // identifier (type name, var name, package alias)
	KindLBRACE    // {
	KindRBRACE    // }
	KindLParen    // (
	KindRParen    // )

	// Template body tokens.
	KindText   // static HTML text
	KindLT     // <
	KindGT     // >
	KindSlash  // /
	KindEQ     // =
	KindString // "..." or '...'
	KindExpr   // { go expr } — the Go expression text

	// @ directives inside template body.
	KindAtIf     // @if
	KindAtElse   // @else
	KindAtFor    // @for
	KindAtSwitch // @switch
	KindAtCase   // @case
	KindAtDefault // @default
	KindAtView   // @component / @view
	KindAtChildren // @children
	KindAtYield  // @yield
	KindAtError  // @error("field")
	KindAtMemo   // @memo(func(rc, props) string { ... })

	// Decorators (before func declarations).
	KindAtOOB     // @oob "slot-id"
	KindAtAsync   // @async
	KindAtFallback // @fallback(ComponentName)

	// Asset directives.
	KindAtCSS     // @css "/path"
	KindAtJS      // @js "/path"
	KindAtAction  // @action name(rc, props) error { ... }

	KindColon     // :
	KindComma     // ,
)

// Token is a span of bytes in the source with a Kind
// classification.
type Token struct {
	Kind  Kind
	Start uint32
	End   uint32
}

func (t Token) String(src []byte) string {
	if t.Start >= t.End || int(t.End) > len(src) {
		return ""
	}
	return string(src[t.Start:t.End])
}

// Scanner tokenizes .gsx source. The scanner is the lexer: it
// reads the source byte by byte using SWAR transitions and
// emits tokens. The parser calls Scan() in a loop.
type Scanner struct {
	src        []byte
	pos        uint32
	end        uint32
	inTemplate bool // true once the func body { is seen
}

// NewScanner returns a Scanner over src.
func NewScanner(src []byte) *Scanner {
	return &Scanner{src: src, pos: 0, end: uint32(len(src))}
}

// EnterTemplate switches the scanner into template-body mode
// where { triggers expression scanning. Call after the func
// body { is consumed.
func (s *Scanner) EnterTemplate() { s.inTemplate = true }

// ExitTemplate returns the scanner to preamble mode — the next
// func's signature and decorators scan idents, not text. Call
// when a func body closes.
func (s *Scanner) ExitTemplate() { s.inTemplate = false }

// NextByte reads the next raw byte and advances the position.
// Returns 0 at EOF.
func (s *Scanner) NextByte() byte {
	if s.pos >= s.end {
		return 0
	}
	b := s.src[s.pos]
	s.pos++
	return b
}

// Pos returns the current position.
func (s *Scanner) Pos() uint32 { return s.pos }

// Scan reads the next token. Returns KindEOF when exhausted.
func (s *Scanner) Scan() Token {
	s.skipWS()
	if s.pos >= s.end {
		return Token{Kind: KindEOF, Start: s.pos, End: s.pos}
	}

	b := s.src[s.pos]

	switch {
	case b == '@':
		return s.scanAtDirective()
	case b == '<':
		return s.scanTag()
	case b == '{':
		if s.inTemplate {
			return s.scanExpr()
		}
		s.pos++
		return Token{Kind: KindLBRACE, Start: s.pos - 1, End: s.pos}
	case b == '"', b == '\'', b == '`':
		return s.scanString(b)
	case b == '}':
		s.pos++
		return Token{Kind: KindRBRACE, Start: s.pos - 1, End: s.pos}
	case b == '>':
		s.pos++
		return Token{Kind: KindGT, Start: s.pos - 1, End: s.pos}
	case b == '/':
		s.pos++
		return Token{Kind: KindSlash, Start: s.pos - 1, End: s.pos}
	case b == '=':
		s.pos++
		return Token{Kind: KindEQ, Start: s.pos - 1, End: s.pos}
	case b == ':':
		s.pos++
		return Token{Kind: KindColon, Start: s.pos - 1, End: s.pos}
	case b == ',':
		s.pos++
		return Token{Kind: KindComma, Start: s.pos - 1, End: s.pos}
	case b == '(':
		s.pos++
		return Token{Kind: KindLParen, Start: s.pos - 1, End: s.pos}
	case b == ')':
		s.pos++
		return Token{Kind: KindRParen, Start: s.pos - 1, End: s.pos}
	case b == 'f':
		if s.inTemplate {
			// "func" in prose is text, not a keyword.
			return s.scanText()
		}
		if s.matchKeyword("func") {
			return s.scanFunc()
		}
		return s.scanIdent()
	case isLetter(b):
		if s.inTemplate {
			// In the body, letter runs are prose — scanText
			// keeps spaces and punctuation together so
			// "Hello world, this is a test." survives whole.
			return s.scanText()
		}
		return s.scanIdent()
	case b == '\n', b == '\r':
		return s.scanText()
	default:
		return s.scanText()
	}
}

// skipWS advances past whitespace and comments.
func (s *Scanner) skipWS() {
	for s.pos < s.end {
		b := s.src[s.pos]
		switch {
		case b == '\n' || b == '\r':
			s.pos++
			if s.inTemplate {
				// Strip the line's indentation too. Mid-line
				// spaces are text ("Hello {x} world"); line
				// starts are layout.
				for s.pos < s.end && (s.src[s.pos] == ' ' || s.src[s.pos] == '\t') {
					s.pos++
				}
			}
		case b == ' ' || b == '\t':
			if s.inTemplate {
				// Mid-line whitespace belongs to the text run —
				// stop so scanText absorbs it.
				return
			}
			s.pos++
		case b == '/' && s.pos+1 < s.end && s.src[s.pos+1] == '/':
			// Line comment — skip to newline.
			s.pos += 2
			for s.pos < s.end && s.src[s.pos] != '\n' && s.src[s.pos] != '\r' {
				s.pos++
			}
		default:
			return
		}
	}
}

func (s *Scanner) scanIdent() Token {
	start := s.pos
	for s.pos < s.end && isIdentByte(s.src[s.pos]) {
		s.pos++
	}
	return Token{Kind: KindIdent, Start: start, End: s.pos}
}

func (s *Scanner) scanText() Token {
	start := s.pos
	// Consume until we hit a trigger character. Quotes are
	// triggers too: directive values like @case "admin" must
	// scan as KindString, and prose quotes round-trip as text
	// via the parser's KindString append.
	for s.pos < s.end {
		b := s.src[s.pos]
		if b == '<' || b == '{' || b == '@' || b == '"' || b == '\'' || b == '`' {
			break
		}
		s.pos++
	}
	if s.pos == start {
		// Don't return empty text — advance past one char.
		s.pos++
		return Token{Kind: KindText, Start: start, End: s.pos}
	}
	return Token{Kind: KindText, Start: start, End: s.pos}
}

func (s *Scanner) scanString(quote byte) Token {
	start := s.pos
	s.pos++ // skip opening quote
	for s.pos < s.end {
		b := s.src[s.pos]
		if b == '\\' {
			s.pos += 2 // skip escaped char
			continue
		}
		if b == quote {
			s.pos++
			break
		}
		s.pos++
	}
	return Token{Kind: KindString, Start: start, End: s.pos}
}

func (s *Scanner) scanExpr() Token {
	start := s.pos
	s.pos++ // skip {
	// Short-circuit: if next char is newline or
	// whitespace + < or @, this { opens a block.
	if s.pos < s.end {
		if b := s.src[s.pos]; b == '\n' || b == '\r' {
			return Token{Kind: KindExpr, Start: start, End: s.pos}
		}
		peek := s.pos
		for peek < s.end && (s.src[peek] == ' ' || s.src[peek] == '\t') {
			peek++
		}
		if peek < s.end && (s.src[peek] == '<' || s.src[peek] == '@') {
			return Token{Kind: KindExpr, Start: start, End: s.pos}
		}
	}
	depth := 1
	for s.pos < s.end && depth > 0 {
		b := s.src[s.pos]
		switch b {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				s.pos++
				return Token{Kind: KindExpr, Start: start, End: s.pos}
			}
		case '"', '\'', '`':
			end := s.skipString(b)
			if end == 0 {
				s.pos++
			}
		case '/':
			if s.pos+1 < s.end {
				if s.src[s.pos+1] == '/' {
					for s.pos < s.end && s.src[s.pos] != '\n' {
						s.pos++
					}
					continue
				}
				if s.src[s.pos+1] == '*' {
					s.pos += 2
					for s.pos+1 < s.end && !(s.src[s.pos] == '*' && s.src[s.pos+1] == '/') {
						s.pos++
					}
					if s.pos+1 < s.end {
						s.pos += 2
					}
					continue
				}
			}
			s.pos++
		default:
			s.pos++
		}
	}
	return Token{Kind: KindExpr, Start: start, End: s.pos}
}

func (s *Scanner) skipString(quote byte) uint32 {
	if s.pos >= s.end {
		return 0
	}
	s.pos++ // skip opening quote
	for s.pos < s.end {
		b := s.src[s.pos]
		if b == '\\' {
			s.pos += 2
			continue
		}
		if b == quote {
			s.pos++
			return s.pos
		}
		s.pos++
	}
	return 0
}

func (s *Scanner) scanTag() Token {
	start := s.pos
	s.pos++ // skip <
	// Consume until > handling nested braces for attributes.
	depth := 0
	for s.pos < s.end {
		b := s.src[s.pos]
		switch b {
		case '>':
			if depth == 0 {
				s.pos++
				return Token{Kind: KindLT, Start: start, End: s.pos}
			}
			s.pos++
		case '{':
			depth++
			s.pos++
		case '}':
			depth--
			s.pos++
		case '"', '\'', '`':
			s.skipString(b)
		default:
			s.pos++
		}
	}
	return Token{Kind: KindLT, Start: start, End: s.pos}
}

func (s *Scanner) scanAtDirective() Token {
	start := s.pos
	s.pos++ // skip @
	// Read the keyword.
	kwStart := s.pos
	for s.pos < s.end && isLetter(s.src[s.pos]) {
		s.pos++
	}
	kw := string(s.src[kwStart:s.pos])

	switch kw {
	case "if":
		return Token{Kind: KindAtIf, Start: start, End: s.pos}
	case "else":
		return Token{Kind: KindAtElse, Start: start, End: s.pos}
	case "for":
		return Token{Kind: KindAtFor, Start: start, End: s.pos}
	case "switch":
		return Token{Kind: KindAtSwitch, Start: start, End: s.pos}
	case "case":
		return Token{Kind: KindAtCase, Start: start, End: s.pos}
	case "default":
		return Token{Kind: KindAtDefault, Start: start, End: s.pos}
	case "component", "view":
		return Token{Kind: KindAtView, Start: start, End: s.pos}
	case "children":
		return Token{Kind: KindAtChildren, Start: start, End: s.pos}
	case "yield":
		return Token{Kind: KindAtYield, Start: start, End: s.pos}
	case "error":
		return Token{Kind: KindAtError, Start: start, End: s.pos}
	case "memo":
		return Token{Kind: KindAtMemo, Start: start, End: s.pos}
	case "oob":
		return Token{Kind: KindAtOOB, Start: start, End: s.pos}
	case "async":
		return Token{Kind: KindAtAsync, Start: start, End: s.pos}
	case "fallback":
		return Token{Kind: KindAtFallback, Start: start, End: s.pos}
	case "css":
		return Token{Kind: KindAtCSS, Start: start, End: s.pos}
	case "js":
		return Token{Kind: KindAtJS, Start: start, End: s.pos}
	case "action":
		return Token{Kind: KindAtAction, Start: start, End: s.pos}
	case "import":
		return Token{Kind: KindAtImport, Start: start, End: s.pos}
	default:
		// Not a directive keyword — the @ is literal text:
		// emails (alice@demo.dev), at-handles (@{user.ID}),
		// @media in prose. Emit just the @ as text and let the
		// next Scan consume what follows.
		s.pos = kwStart
		return Token{Kind: KindText, Start: start, End: kwStart}
	}
}

func (s *Scanner) scanFunc() Token {
	start := s.pos
	// We already matched "func" — skip past it.
	s.pos += 4
	return Token{Kind: KindFunc, Start: start, End: s.pos}
}

func (s *Scanner) matchKeyword(kw string) bool {
	end := s.pos + uint32(len(kw))
	if end > s.end {
		return false
	}
	for i := 0; i < len(kw); i++ {
		if s.src[s.pos+uint32(i)] != kw[i] {
			return false
		}
	}
	// Must be followed by non-ident char.
	if end < s.end && isIdentByte(s.src[end]) {
		return false
	}
	return true
}

func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_'
}

func isIdentByte(b byte) bool {
	return isLetter(b) || (b >= '0' && b <= '9') || b == '.'
}
