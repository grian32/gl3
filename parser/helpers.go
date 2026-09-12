package parser

import (
	"fmt"
	"gl3/lexer"
	"gl3/util"
)

func (p *Parser) peekPrecedence() byte {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}

	return LOWEST
}

func (p *Parser) currPrecedence() byte {
	if p, ok := precedences[p.currToken.Type]; ok {
		return p
	}

	return LOWEST
}

func (p *Parser) noPrefixParseFnError(t lexer.Token, pos *util.Position) {
	p.appendError(pos, "no prefix parse function for %s, peek=%s found", t.Type, p.peekToken.Type)
	// to prevent inf loops
	p.NextToken()
}

func (p *Parser) expectPeek(t lexer.TokenType) bool {
	if p.peekTokenIs(t) {
		p.NextToken()
		return true
	}

	p.peekError(t, &p.peekToken.Position)
	return false
}

func (p *Parser) peekError(t lexer.TokenType, pos *util.Position) {
	p.appendError(pos, "expected next token to be %s, got %s instead", t, p.peekToken.Type)
}

func (p *Parser) expectCurr(t lexer.TokenType) bool {
	if p.currTokenIs(t) {
		p.NextToken()
		return true
	}

	p.currError(t, &p.currToken.Position)
	// advance so it can keep parsing instead of a loop or something
	p.NextToken()
	return false
}

func (p *Parser) currError(t lexer.TokenType, pos *util.Position) {
	p.appendError(pos, "expected curr token to be %s, got %s instead", t, p.peekToken.Type)
}

func (p *Parser) appendError(pos *util.Position, msg string, v ...any) {
	p.Errors = append(p.Errors, util.PositionError{
		Position: pos,
		Msg:      fmt.Sprintf(msg, v...),
	})
}

func (p *Parser) peekTokenIs(tt lexer.TokenType) bool {
	return p.peekToken.Type == tt
}

func (p *Parser) currTokenIs(tt lexer.TokenType) bool {
	return p.currToken.Type == tt
}

func (p *Parser) parseType() (lexer.VarType, bool) {
	var vt lexer.VarType
	if p.currTokenIs(lexer.TYPE) {
		vt = p.currToken.VarType
	} else if p.currTokenIs(lexer.IDENTIFIER) {
		vt.IsStructType = true
		vt.StructName = p.currToken.Literal
	} else {
		return lexer.VarType{}, false
	}
	p.NextToken()
	if !p.currTokenIs(lexer.ASTERISK) {
		return vt, true
	}

	for p.currTokenIs(lexer.ASTERISK) {
		vt.Pointer++
		p.NextToken()
	}

	return vt, true
}

// vt, ident, typeOk, identOk, its kinda messy on the user end with two bool but it leads to better rr msgs so its worth..
func (p *Parser) parseTypedIdentifier() (lexer.VarType, *IdentifierExpression, bool, bool) {
	vt, ok := p.parseType()
	if !ok {
		return lexer.VarType{}, nil, false, true
	}
	if !p.currTokenIs(lexer.IDENTIFIER) {
		return lexer.VarType{}, nil, true, false
	}
	ident := &IdentifierExpression{Token: p.currToken, Value: p.currToken.Literal}
	p.NextToken()

	return vt, ident, true, true
}

func (p *Parser) parseExpressionList(end lexer.TokenType) ([]Expression, bool) {
	var params []Expression
	for !p.currTokenIs(end) {
		expr := p.parseExpression(LOWEST)
		params = append(params, expr)
		if p.currTokenIs(end) {
			break
		} else if p.currTokenIs(lexer.COMMA) {
			p.NextToken()
			continue
		} else {
			return nil, false
		}
	}

	return params, true
}
