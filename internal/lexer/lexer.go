// Package lexer scans markdown byte streams into a token stream.
//
// The lexer is SWAR-driven: it classifies 8 bytes per cycle via bitwise
// operations on uint64. See swar.go for FindByte, FindByteNot, and SkipWS
// — the hot-path helpers used by the parser. The token-stream scaffold
// below is unused by the parser today and is preserved for the upcoming
// tokenize-first iteration.
package lexer

// Kind enumerates token types emitted by the lexer.
type Kind uint8

const (
	KindEOF Kind = iota
	KindText
	KindNewline
	KindHeading
	KindEmphasis
	KindStrong
	KindCode
	KindLink
	KindImage
	KindListBullet
	KindListNumber
	KindBlockquote
	KindCodeFence
	KindHr
	KindTable
	KindRawHTML
	KindExtension
)

// Token is a single lexer emission. Start/End are byte offsets into the
// input slice — no copy.
type Token struct {
	Start uint32
	End   uint32
	Kind  Kind
}

// Scanner holds lexer state.
type Scanner struct {
	src []byte
	pos uint32
}

// New returns a Scanner over src. The scanner keeps a slice header only;
// it does not retain src beyond the call.
func New(src []byte) *Scanner {
	return &Scanner{src: src}
}

// Next advances and returns the next token. Returns (Token{}, false) on EOF.
func (s *Scanner) Next() (Token, bool) {
	if int(s.pos) >= len(s.src) {
		return Token{Kind: KindEOF}, false
	}
	start := s.pos
	if s.src[s.pos] == '\n' {
		s.pos++
		return Token{Start: start, End: s.pos, Kind: KindNewline}, true
	}
	for int(s.pos) < len(s.src) && s.src[s.pos] != '\n' {
		s.pos++
	}
	return Token{Start: start, End: s.pos, Kind: KindText}, true
}
