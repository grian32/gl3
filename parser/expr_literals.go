package parser

// GET RID OF LATER!

import (
	"gl3/lexer"
	"strconv"
)

func (p *Parser) parseIdentifier() Expression {
	expr := &IdentifierExpression{Token: p.currToken, Value: p.currToken.Literal}
	p.NextToken()
	return expr
}

func (p *Parser) parseGroupedExpression() Expression {
	p.NextToken()

	exp := p.parseExpression(LOWEST)

	if !p.expectCurr(lexer.RPAREN) {
		return nil
	}

	return exp
}

func (p *Parser) parseIntegerLiteral() Expression {
	vt := lexer.VarType{Base: lexer.Int, Pointer: 0}
	lit := &IntegerLiteral{Token: p.currToken, Type: vt}

	// could be more efficient by parsing based on type or the lack thereof but this makes for a decent chunk cleaner
	// code
	uvalue, err := strconv.ParseUint(p.currToken.Literal, 0, 64)
	if err != nil {
		p.appendError(&p.currToken.Position, "could not parse %q as integer", p.currToken.Literal)
	}

	lit.UValue = uvalue

	p.NextToken()
	if p.currTokenIs(lexer.IDENTIFIER) {
		switch p.currToken.Literal {
		case "i32":
			lit.Type.Base = lexer.Int32
		case "i16":
			lit.Type.Base = lexer.Int16
		case "i8":
			lit.Type.Base = lexer.Int8
		case "u32":
			lit.Type.Base = lexer.Uint32
		case "u16":
			lit.Type.Base = lexer.Uint16
		case "u8":
			lit.Type.Base = lexer.Uint8
		case "u64":
			lit.Type.Base = lexer.Uint
		default:
			lit.Type.Base = lexer.None
			p.appendError(&p.currToken.Position, "unknown integer literal suffix %s", p.currToken.Literal)
		}
		p.NextToken()
	}

	return lit
}

func (p *Parser) parseStringLiteral() Expression {
	expr := &StringLiteral{Token: p.currToken, Value: p.currToken.Literal + "\000"}
	p.NextToken()
	return expr
}

func (p *Parser) parseFloatLiteral() Expression {
	vt := lexer.VarType{Base: lexer.Float, Pointer: 0}
	lit := &FloatLiteral{Token: p.currToken, Type: vt}

	value, err := strconv.ParseFloat(p.currToken.Literal, 32)
	if err != nil {
		p.appendError(&p.currToken.Position, "could not parse %q as float", p.currToken.Literal)
	}

	lit.Value = float32(value)
	p.NextToken()

	return lit
}

func (p *Parser) parseCharLiteral() Expression {
	vt := lexer.VarType{Base: lexer.Int8, Pointer: 0}
	expr := &IntegerLiteral{Token: p.currToken, UValue: uint64(p.currToken.Literal[0]), Type: vt}
	p.NextToken()
	return expr
}

func (p *Parser) parseBoolean() Expression {
	expr := &BooleanExpression{Token: p.currToken}

	if p.currTokenIs(lexer.TRUE) {
		expr.Value = true
	} else {
		expr.Value = false
	}
	p.NextToken()

	return expr
}
