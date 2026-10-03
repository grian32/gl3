package lexer

import (
	"slices"
	"testing"
)

func lexTypes(src string) ([]TokenType, []string) {
	l := New(src)
	var types []TokenType
	var literals []string
	for tok := l.NextToken(); tok.Type != EOF; tok = l.NextToken() {
		types = append(types, tok.Type)
		literals = append(literals, tok.Literal)
	}
	return types, literals
}

func TestOperatorTokens(t *testing.T) {
	tests := []struct {
		src  string
		want []TokenType
	}{
		// Multi-character operators must not swallow the character after them.
		{"a==b", []TokenType{IDENTIFIER, EQ, IDENTIFIER}},
		{"a!=b", []TokenType{IDENTIFIER, NOTEQ, IDENTIFIER}},
		{"a<=b", []TokenType{IDENTIFIER, LTEQ, IDENTIFIER}},
		{"a>=b", []TokenType{IDENTIFIER, GTEQ, IDENTIFIER}},
		{"a&&b", []TokenType{IDENTIFIER, LAND, IDENTIFIER}},
		{"a||b", []TokenType{IDENTIFIER, LOR, IDENTIFIER}},
		{"a<<b", []TokenType{IDENTIFIER, SHL, IDENTIFIER}},
		{"a>>b", []TokenType{IDENTIFIER, SHR, IDENTIFIER}},
		{")->b", []TokenType{RPAREN, ARROW, IDENTIFIER}},
		{"a+=b", []TokenType{IDENTIFIER, PLUS_ASSIGN, IDENTIFIER}},
		{"a-=b", []TokenType{IDENTIFIER, MINUS_ASSIGN, IDENTIFIER}},
		{"a*=b", []TokenType{IDENTIFIER, ASTERISK_ASSIGN, IDENTIFIER}},
		{"a/=b", []TokenType{IDENTIFIER, SLASH_ASSIGN, IDENTIFIER}},
		{"a%=b", []TokenType{IDENTIFIER, PERCENT_ASSIGN, IDENTIFIER}},
		{"a&=b", []TokenType{IDENTIFIER, AMPERSAND_ASSIGN, IDENTIFIER}},
		{"a|=b", []TokenType{IDENTIFIER, PIPE_ASSIGN, IDENTIFIER}},
		{"a^=b", []TokenType{IDENTIFIER, CARET_ASSIGN, IDENTIFIER}},
		{"a<<=b", []TokenType{IDENTIFIER, SHL_ASSIGN, IDENTIFIER}},
		{"a>>=b", []TokenType{IDENTIFIER, SHR_ASSIGN, IDENTIFIER}},
		{"a+b", []TokenType{IDENTIFIER, PLUS, IDENTIFIER}},
		{"a&b", []TokenType{IDENTIFIER, AMPERSAND, IDENTIFIER}},
		{"a|b", []TokenType{IDENTIFIER, PIPE, IDENTIFIER}},
		{"a^b", []TokenType{IDENTIFIER, CARET, IDENTIFIER}},
		{"~a", []TokenType{TILDE, IDENTIFIER}},
		{"a=-b", []TokenType{IDENTIFIER, ASSIGN, MINUS, IDENTIFIER}},
		{"a=&b", []TokenType{IDENTIFIER, ASSIGN, AMPERSAND, IDENTIFIER}},
		{"a<-b", []TokenType{IDENTIFIER, LT, MINUS, IDENTIFIER}},
		{"a< <b", []TokenType{IDENTIFIER, LT, LT, IDENTIFIER}},
		{"a<< =b", []TokenType{IDENTIFIER, SHL, ASSIGN, IDENTIFIER}},
		{"a/b//c", []TokenType{IDENTIFIER, SLASH, IDENTIFIER}},
	}
	for _, tt := range tests {
		got, literals := lexTypes(tt.src)
		if !slices.Equal(got, tt.want) {
			t.Errorf("%q: got %v (%q), want %v", tt.src, got, literals, tt.want)
		}
	}
}

func TestOperatorTokenPosition(t *testing.T) {
	l := New("a <<= b")
	l.NextToken()
	tok := l.NextToken()
	if tok.Literal != "<<=" {
		t.Fatalf("literal = %q, want %q", tok.Literal, "<<=")
	}
	p := tok.Position
	if p.StartLine != 1 || p.EndLine != 1 || p.StartCol != 3 || p.EndCol != 5 {
		t.Errorf("position = %+v, want line 1, cols 3-5", p)
	}
}
