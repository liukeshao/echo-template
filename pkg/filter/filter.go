// Package filter provides a parser for AIP-160 filter expressions.
//
// Supported syntax (subset of the full AIP-160 grammar):
//
//	filter     = expr EOF
//	expr       = and_term ( "AND" and_term )*
//	and_term   = or_term  ( "OR"  or_term  )*        -- OR has higher precedence
//	or_term    = "NOT" or_term | "-" or_term | primary
//	primary    = "(" expr ")" | restriction
//	restriction = field ( comparator | ":" ) value
//	field       = IDENT ( "." IDENT )*
//	comparator  = "=" | "!=" | "<" | "<=" | ">" | ">="
//	value       = STRING | NUMBER | BOOL | "*" | IDENT
//
// Examples:
//
//	title = "Go"
//	price < 50 AND author = "Bob"
//	NOT (title : "test") OR price >= 10
//	title = "Go*"                          -- wildcard: starts with "Go"
package filter

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ---- Public AST types -------------------------------------------------------

// Expr is the interface implemented by all AST nodes.
type Expr interface{ exprNode() }

// AndExpr represents left AND right.
type AndExpr struct{ Left, Right Expr }

func (*AndExpr) exprNode() {}

// OrExpr represents left OR right.
type OrExpr struct{ Left, Right Expr }

func (*OrExpr) exprNode() {}

// NotExpr represents NOT operand or -operand.
type NotExpr struct{ Operand Expr }

func (*NotExpr) exprNode() {}

// RestrictionExpr represents field op value.
//
//	Field is a dot-joined path, e.g. "title" or "author.name".
//	Op is one of the Op* constants.
//	Value is one of StringValue, NumberValue, BoolValue, WildcardValue, TextValue.
type RestrictionExpr struct {
	Field string
	Op    Op
	Value Value
}

func (*RestrictionExpr) exprNode() {}

// Op is a comparison operator.
type Op int

const (
	OpEQ  Op = iota // =
	OpNEQ           // !=
	OpLT            // <
	OpLTE           // <=
	OpGT            // >
	OpGTE           // >=
	OpHas           // : (has / contains)
)

func (o Op) String() string {
	return [...]string{"=", "!=", "<", "<=", ">", ">=", ":"}[o]
}

// ---- Value types ------------------------------------------------------------

// Value is the interface implemented by all right-hand-side value nodes.
type Value interface{ valueNode() }

// StringValue is a double- or single-quoted string, possibly with wildcards.
type StringValue struct {
	V string
}

func (*StringValue) valueNode() {}

// NumberValue is an integer or floating-point literal.
type NumberValue struct{ V float64 }

func (*NumberValue) valueNode() {}

// BoolValue is the literal true or false.
type BoolValue struct{ V bool }

func (*BoolValue) valueNode() {}

// WildcardValue represents a standalone "*".
type WildcardValue struct{}

func (*WildcardValue) valueNode() {}

// TextValue is an unquoted bare word on the right-hand side (e.g. an enum name).
type TextValue struct{ V string }

func (*TextValue) valueNode() {}

// ---- Public API -------------------------------------------------------------

// Parse parses an AIP-160 filter string and returns the root expression.
// Returns nil, nil for an empty or whitespace-only filter.
func Parse(s string) (Expr, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	tokens, err := tokenize(s)
	if err != nil {
		return nil, fmt.Errorf("filter: lex error: %w", err)
	}
	p := &parser{tokens: tokens}
	expr, err := p.parseExpr()
	if err != nil {
		return nil, fmt.Errorf("filter: parse error: %w", err)
	}
	if p.peek().kind != kEOF {
		return nil, fmt.Errorf("filter: unexpected token %q after expression", p.peek().raw)
	}
	return expr, nil
}

// ---- Token types ------------------------------------------------------------

type tokenKind int

const (
	kEOF    tokenKind = iota
	kIdent            // identifier or keyword (AND, OR, NOT, true, false)
	kString           // quoted string literal (value already unescaped)
	kNumber           // integer or float
	kLParen           // (
	kRParen           // )
	kEq               // =
	kNeq              // !=
	kLt               // <
	kLte              // <=
	kGt               // >
	kGte              // >=
	kColon            // :
	kDot              // .
	kMinus            // -
	kStar             // *
)

type tok struct {
	kind tokenKind
	raw  string // unescaped for strings
	pos  int    // rune position in original input (for error messages)
}

// ---- Lexer ------------------------------------------------------------------

func tokenize(s string) ([]tok, error) {
	r := []rune(s)
	var tokens []tok
	i := 0

	for i < len(r) {
		pos := i

		switch {
		case unicode.IsSpace(r[i]):
			i++
			continue

		case r[i] == '(':
			tokens = append(tokens, tok{kLParen, "(", pos})
			i++

		case r[i] == ')':
			tokens = append(tokens, tok{kRParen, ")", pos})
			i++

		case r[i] == '*':
			tokens = append(tokens, tok{kStar, "*", pos})
			i++

		case r[i] == '.':
			tokens = append(tokens, tok{kDot, ".", pos})
			i++

		case r[i] == ':':
			tokens = append(tokens, tok{kColon, ":", pos})
			i++

		case r[i] == '=':
			tokens = append(tokens, tok{kEq, "=", pos})
			i++

		case r[i] == '!':
			if i+1 < len(r) && r[i+1] == '=' {
				tokens = append(tokens, tok{kNeq, "!=", pos})
				i += 2
			} else {
				return nil, fmt.Errorf("position %d: unexpected '!', did you mean '!='?", pos)
			}

		case r[i] == '<':
			if i+1 < len(r) && r[i+1] == '=' {
				tokens = append(tokens, tok{kLte, "<=", pos})
				i += 2
			} else {
				tokens = append(tokens, tok{kLt, "<", pos})
				i++
			}

		case r[i] == '>':
			if i+1 < len(r) && r[i+1] == '=' {
				tokens = append(tokens, tok{kGte, ">=", pos})
				i += 2
			} else {
				tokens = append(tokens, tok{kGt, ">", pos})
				i++
			}

		case r[i] == '-':
			tokens = append(tokens, tok{kMinus, "-", pos})
			i++

		case r[i] == '"' || r[i] == '\'':
			t, n, err := lexString(r, i)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, t)
			i += n

		case unicode.IsDigit(r[i]):
			t, n := lexNumber(r, i, pos)
			tokens = append(tokens, t)
			i += n

		case unicode.IsLetter(r[i]) || r[i] == '_':
			t, n := lexIdent(r, i, pos)
			tokens = append(tokens, t)
			i += n

		default:
			return nil, fmt.Errorf("position %d: unexpected character %q", pos, r[i])
		}
	}

	tokens = append(tokens, tok{kEOF, "", len(r)})
	return tokens, nil
}

// lexString reads a quoted string; returns the token and the number of runes consumed.
func lexString(r []rune, start int) (tok, int, error) {
	quote := r[start]
	i := start + 1
	var sb strings.Builder
	for i < len(r) && r[i] != quote {
		if r[i] == '\\' && i+1 < len(r) {
			i++
			switch r[i] {
			case '"', '\'', '\\':
				sb.WriteRune(r[i])
			case 'n':
				sb.WriteRune('\n')
			case 't':
				sb.WriteRune('\t')
			default:
				sb.WriteRune('\\')
				sb.WriteRune(r[i])
			}
		} else {
			sb.WriteRune(r[i])
		}
		i++
	}
	if i >= len(r) {
		return tok{}, 0, fmt.Errorf("position %d: unterminated string", start)
	}
	v := sb.String()
	return tok{kString, v, start}, i - start + 1, nil
}

// lexNumber reads a numeric literal; returns the token and runes consumed.
func lexNumber(r []rune, start, pos int) (tok, int) {
	i := start
	for i < len(r) && unicode.IsDigit(r[i]) {
		i++
	}
	if i < len(r) && r[i] == '.' {
		i++
		for i < len(r) && unicode.IsDigit(r[i]) {
			i++
		}
	}
	if i < len(r) && (r[i] == 'e' || r[i] == 'E') {
		i++
		if i < len(r) && (r[i] == '+' || r[i] == '-') {
			i++
		}
		for i < len(r) && unicode.IsDigit(r[i]) {
			i++
		}
	}
	return tok{kNumber, string(r[start:i]), pos}, i - start
}

// lexIdent reads an identifier; returns the token and runes consumed.
func lexIdent(r []rune, start, pos int) (tok, int) {
	i := start
	for i < len(r) && (unicode.IsLetter(r[i]) || unicode.IsDigit(r[i]) || r[i] == '_') {
		i++
	}
	return tok{kIdent, string(r[start:i]), pos}, i - start
}

// ---- Parser -----------------------------------------------------------------

type parser struct {
	tokens []tok
	pos    int
}

func (p *parser) peek() tok {
	if p.pos >= len(p.tokens) {
		return tok{kEOF, "", -1}
	}
	return p.tokens[p.pos]
}

func (p *parser) consume() tok {
	t := p.peek()
	if t.kind != kEOF {
		p.pos++
	}
	return t
}

// isKeyword reports whether ident t is the keyword kw (case-insensitive).
func isKeyword(t tok, kw string) bool {
	return t.kind == kIdent && strings.EqualFold(t.raw, kw)
}

// parseExpr parses: and_term ( "AND" and_term )*
func (p *parser) parseExpr() (Expr, error) {
	left, err := p.parseAndTerm()
	if err != nil {
		return nil, err
	}
	for isKeyword(p.peek(), "AND") {
		p.consume()
		right, err := p.parseAndTerm()
		if err != nil {
			return nil, err
		}
		left = &AndExpr{Left: left, Right: right}
	}
	return left, nil
}

// parseAndTerm parses: or_term ( "OR" or_term )*  [OR has higher precedence than AND]
func (p *parser) parseAndTerm() (Expr, error) {
	left, err := p.parseOrTerm()
	if err != nil {
		return nil, err
	}
	for isKeyword(p.peek(), "OR") {
		p.consume()
		right, err := p.parseOrTerm()
		if err != nil {
			return nil, err
		}
		left = &OrExpr{Left: left, Right: right}
	}
	return left, nil
}

// parseOrTerm parses: "NOT" or_term | "-" or_term | primary
func (p *parser) parseOrTerm() (Expr, error) {
	t := p.peek()
	if isKeyword(t, "NOT") {
		p.consume()
		operand, err := p.parseOrTerm()
		if err != nil {
			return nil, err
		}
		return &NotExpr{Operand: operand}, nil
	}
	if t.kind == kMinus {
		p.consume()
		operand, err := p.parseOrTerm()
		if err != nil {
			return nil, err
		}
		return &NotExpr{Operand: operand}, nil
	}
	return p.parsePrimary()
}

// parsePrimary parses: "(" expr ")" | restriction
func (p *parser) parsePrimary() (Expr, error) {
	t := p.peek()
	if t.kind == kLParen {
		p.consume()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.peek().kind != kRParen {
			return nil, fmt.Errorf("position %d: expected ')', got %q", p.peek().pos, p.peek().raw)
		}
		p.consume()
		return expr, nil
	}
	return p.parseRestriction()
}

// parseRestriction parses: field ( comparator | ":" ) value
//
// field = IDENT ( "." IDENT )*
func (p *parser) parseRestriction() (Expr, error) {
	t := p.peek()
	if t.kind != kIdent {
		return nil, fmt.Errorf("position %d: expected field name, got %q", t.pos, t.raw)
	}
	if isKeyword(t, "AND") || isKeyword(t, "OR") || isKeyword(t, "NOT") {
		return nil, fmt.Errorf("position %d: unexpected keyword %q where field name expected", t.pos, t.raw)
	}
	p.consume()
	var field strings.Builder
	field.WriteString(t.raw)

	// Dotted traversal: field = IDENT ( "." IDENT )*
	for p.peek().kind == kDot {
		p.consume()
		next := p.peek()
		if next.kind != kIdent {
			return nil, fmt.Errorf("position %d: expected field name after '.', got %q", next.pos, next.raw)
		}
		p.consume()
		field.WriteString("." + next.raw)
	}

	op, err := p.parseOp()
	if err != nil {
		return nil, fmt.Errorf("field %q: %w", field.String(), err)
	}

	val, err := p.parseValue()
	if err != nil {
		return nil, fmt.Errorf("field %q: %w", field.String(), err)
	}

	return &RestrictionExpr{Field: field.String(), Op: op, Value: val}, nil
}

func (p *parser) parseOp() (Op, error) {
	t := p.consume()
	switch t.kind {
	case kEq:
		return OpEQ, nil
	case kNeq:
		return OpNEQ, nil
	case kLt:
		return OpLT, nil
	case kLte:
		return OpLTE, nil
	case kGt:
		return OpGT, nil
	case kGte:
		return OpGTE, nil
	case kColon:
		return OpHas, nil
	default:
		return 0, fmt.Errorf("position %d: expected comparison operator (=, !=, <, <=, >, >=, :), got %q", t.pos, t.raw)
	}
}

func (p *parser) parseValue() (Value, error) {
	t := p.consume()
	switch t.kind {
	case kString:
		return &StringValue{V: t.raw}, nil

	case kMinus:
		next := p.consume()
		if next.kind != kNumber {
			return nil, fmt.Errorf("position %d: expected number after '-', got %q", next.pos, next.raw)
		}
		v, err := strconv.ParseFloat("-"+next.raw, 64)
		if err != nil {
			return nil, fmt.Errorf("position %d: invalid number %q: %w", t.pos, "-"+next.raw, err)
		}
		return &NumberValue{V: v}, nil

	case kNumber:
		v, err := strconv.ParseFloat(t.raw, 64)
		if err != nil {
			return nil, fmt.Errorf("position %d: invalid number %q: %w", t.pos, t.raw, err)
		}
		return &NumberValue{V: v}, nil

	case kStar:
		return &WildcardValue{}, nil

	case kIdent:
		switch strings.ToLower(t.raw) {
		case "true":
			return &BoolValue{V: true}, nil
		case "false":
			return &BoolValue{V: false}, nil
		default:
			return &TextValue{V: t.raw}, nil
		}

	default:
		return nil, fmt.Errorf("position %d: expected value (string, number, bool, identifier), got %q", t.pos, t.raw)
	}
}
