package lexer

import (
	"gl3/util"
)

type Lexer struct {
	input   string
	pos     int
	readPos int
	ch      byte

	currLine uint32
	currCh   uint32
}

func New(input string) *Lexer {
	l := &Lexer{input: input, currLine: 1}
	l.readChar()

	return l
}

func (l *Lexer) readChar() {
	if l.readPos >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPos]
	}

	l.pos = l.readPos
	l.readPos += 1
	l.currCh += 1
}

func (l *Lexer) peekChar() byte {
	if l.readPos >= len(l.input) {
		return 0
	} else {
		return l.input[l.readPos]
	}
}

var singleCharToken = map[byte]TokenType{
	'~': TILDE,
	';': SEMICOLON,
	'(': LPAREN,
	')': RPAREN,
	'{': LBRACE,
	'}': RBRACE,
	'[': LBRACKET,
	']': RBRACKET,
	',': COMMA,
	':': COLON,
}

func (l *Lexer) NextToken() Token {
	var tok Token

	for {
		for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
			if l.ch == '\n' {
				l.currLine++
				l.currCh = 0
			}
			l.readChar()
		}

		if l.ch == '/' && l.peekChar() == '/' {
			for l.ch != '\n' && l.ch != 0 {
				l.readChar()
			}

			continue
		}

		break
	}

	sct, ok := singleCharToken[l.ch]
	if ok {
		tok = newToken(sct, l.ch, l.currLine, l.currCh)
		// ret early here is a bit of future proofing/opti
		l.readChar()
		return tok
	}

	switch l.ch {
	case '+':
		tok = l.operatorToken(PLUS)
		l.extendToken(&tok, '=', PLUS_ASSIGN)
	case '*':
		tok = l.operatorToken(ASTERISK)
		l.extendToken(&tok, '=', ASTERISK_ASSIGN)
	case '/':
		tok = l.operatorToken(SLASH)
		l.extendToken(&tok, '=', SLASH_ASSIGN)
	case '%':
		tok = l.operatorToken(PERCENT)
		l.extendToken(&tok, '=', PERCENT_ASSIGN)
	case '^':
		tok = l.operatorToken(CARET)
		l.extendToken(&tok, '=', CARET_ASSIGN)
	case '-':
		tok = l.operatorToken(MINUS)
		if !l.extendToken(&tok, '>', ARROW) {
			l.extendToken(&tok, '=', MINUS_ASSIGN)
		}
	case '&':
		tok = l.operatorToken(AMPERSAND)
		if !l.extendToken(&tok, '&', LAND) {
			l.extendToken(&tok, '=', AMPERSAND_ASSIGN)
		}
	case '|':
		tok = l.operatorToken(PIPE)
		if !l.extendToken(&tok, '|', LOR) {
			l.extendToken(&tok, '=', PIPE_ASSIGN)
		}
	case '=':
		tok = l.operatorToken(ASSIGN)
		l.extendToken(&tok, '=', EQ)
	case '!':
		tok = l.operatorToken(NOT)
		l.extendToken(&tok, '=', NOTEQ)
	case '<':
		tok = l.operatorToken(LT)
		if l.extendToken(&tok, '<', SHL) {
			l.extendToken(&tok, '=', SHL_ASSIGN)
		} else {
			l.extendToken(&tok, '=', LTEQ)
		}
	case '>':
		tok = l.operatorToken(GT)
		if l.extendToken(&tok, '>', SHR) {
			l.extendToken(&tok, '=', SHR_ASSIGN)
		} else {
			l.extendToken(&tok, '=', GTEQ)
		}
	case '.':
		if l.peekChar() == '.' && len(l.input) > l.readPos+1 && l.input[l.readPos+1] == '.' {
			tok.Position = util.Position{
				StartLine: l.currLine,
				StartCol:  l.currCh,
			}
			l.readChar()
			l.readChar()
			tok.Literal = "..."
			tok.Type = ELLIPSIS
			tok.Position.EndLine = l.currLine
			tok.Position.EndCol = l.currCh
		} else {
			tok.Literal = "."
			tok.Type = DOT
			tok.Position = util.Position{
				StartLine: l.currLine,
				StartCol:  l.currCh,
				EndLine:   l.currLine,
				EndCol:    l.currCh,
			}
		}

	case 0:
		tok.Literal = ""
		tok.Type = EOF
		tok.Position = util.Position{
			StartLine: l.currLine,
			StartCol:  l.currCh,
			EndLine:   l.currLine,
			EndCol:    l.currCh,
		}
	default:
		if l.ch == '0' && l.peekChar() == 'x' {
			tok.Position = util.Position{
				StartLine: l.currLine,
				StartCol:  l.currCh,
				EndLine:   l.currLine,
			}
			tok.Literal = l.readHexaInt()
			tok.Suffix = l.readSuffix()
			tok.Type = INT
			tok.Position.EndCol = l.currCh
			return tok
		}

		if util.IsDigit(l.ch) {
			tok.Position = util.Position{
				StartLine: l.currLine,
				StartCol:  l.currCh,
				EndLine:   l.currLine,
			}
			tok.Literal, tok.Type = l.readNumber()
			tok.Suffix = l.readSuffix()
			tok.Position.EndCol = l.currCh
			return tok
		}

		if util.IsAlpha(l.ch) {
			tok.Position = util.Position{
				StartLine: l.currLine,
				StartCol:  l.currCh,
				EndLine:   l.currLine,
			}
			l.readChar()
			tok.Literal = l.readIdentifier()
			tok.Type, tok.VarType.Base = identLookup(tok.Literal)
			tok.Position.EndCol = l.currCh
			return tok
		}
		if l.ch == '"' {
			tok.Position = util.Position{
				StartLine: l.currLine,
				StartCol:  l.currCh,
				EndLine:   l.currLine,
			}
			l.readChar()
			tok.Type = STRING
			//tok.VarType = VarType{Base: Int8, Pointer: 1};
			tok.Literal = l.readString()
			tok.Position.EndCol = l.currCh
		}

		if l.ch == '\'' {
			tok.Position = util.Position{
				StartLine: l.currLine,
				StartCol:  l.currCh,
				EndLine:   l.currLine,
			}
			l.readChar()
			tok.Type = CHAR
			if l.ch == '\'' {
				tok.Literal = ""
			} else {
				tok.Literal = string(l.ch)
				l.readChar() // skip next '
			}
			tok.Position.EndCol = l.currCh
		}
	}

	l.readChar()
	return tok
}

func (l *Lexer) readString() string {
	buf := make([]rune, 0, 16)

	for l.ch != '"' {
		// NOTE: no fucking clue why this results in an infinite loop with an unterminated string lit??, readchar should set it to 0 which != '"' and it should stop?? god knows
		if l.ch == 0 {
			break
		}
		if l.ch == '\\' {
			l.readChar()
			switch l.ch {
			case 'n':
				buf = append(buf, '\n')
			case 't':
				buf = append(buf, '\t')
			case 'r':
				buf = append(buf, '\r')
			case '\\':
				buf = append(buf, '\\')
			case '"':
				buf = append(buf, '"')
			}
		} else {
			buf = append(buf, rune(l.ch))
		}
		l.readChar()
	}

	return string(buf)
}

func (l *Lexer) readSuffix() string {
	start := l.pos
	for util.IsAlphaNumeric(l.ch) {
		l.readChar()
	}
	return l.input[start:l.pos]
}

func (l *Lexer) readHexaInt() string {
	startPos := l.pos
	l.readChar()
	l.readChar()

	for util.IsHexaNumeral(l.ch) {
		l.readChar()
	}

	return l.input[startPos:l.pos]
}

func (l *Lexer) readNumber() (string, TokenType) {
	startPos := l.pos
	tt := INT

	for util.IsDigit(l.ch) || l.ch == '.' {
		if l.ch == '.' {
			tt = FLOAT
		}
		l.readChar()
	}

	return l.input[startPos:l.pos], tt
}

func (l *Lexer) readIdentifier() string {
	startPos := l.pos - 1

	for util.IsAlphaNumeric(l.ch) {
		l.readChar()
	}

	return l.input[startPos:l.pos]
}

func newToken(tt TokenType, ch byte, currLine, currCh uint32) Token {
	return Token{Type: tt, Literal: string(ch), Position: util.Position{
		StartLine: currLine,
		EndLine:   currLine,
		StartCol:  currCh,
		EndCol:    currCh,
	}}
}

func (l *Lexer) operatorToken(tt TokenType) Token {
	return newToken(tt, l.ch, l.currLine, l.currCh)
}

// extendToken grows tok by one character when the next character is next,
// leaving l.ch on the token's last character for NextToken's final readChar.
func (l *Lexer) extendToken(tok *Token, next byte, tt TokenType) bool {
	if l.peekChar() != next {
		return false
	}
	l.readChar()
	tok.Type = tt
	tok.Literal += string(next)
	tok.Position.EndCol = l.currCh
	return true
}

func identLookup(lit string) (TokenType, BaseVarType) {
	switch lit {
	case "int":
		return TYPE, Int
	case "int32":
		return TYPE, Int32
	case "int16":
		return TYPE, Int16
	case "int8":
		return TYPE, Int8
	case "uint":
		return TYPE, Uint
	case "uint8":
		return TYPE, Uint8
	case "uint16":
		return TYPE, Uint16
	case "uint32":
		return TYPE, Uint32
	case "none":
		return TYPE, Void
	case "def":
		return DEF, None
	case "fnc":
		return FNC, None
	case "return":
		return RETURN, None
	case "bool":
		return TYPE, Bool
	case "true":
		return TRUE, None
	case "false":
		return FALSE, None
	case "float":
		return TYPE, Float
	case "as":
		return AS, None
	case "sizeof":
		return SIZEOF, None
	case "vararg":
		return VARARG, None
	case "import":
		return IMPORT, None
	case "char":
		return TYPE, Char
	case "if":
		return IF, None
	case "else":
		return ELSE, None
	case "while":
		return WHILE, None
	case "struct":
		return STRUCT, None
	case "break":
		return BREAK, None
	case "continue":
		return CONTINUE, None
	case "global":
		return GLOBAL, None
	case "const":
		return CONST, None
	case "extern":
		return EXTERN, None
	case "private":
		return PRIVATE, None
	case "nullptr":
		return NULLPTR, None
	case "float32":
		return TYPE, Float32
	}

	return IDENTIFIER, None
}
