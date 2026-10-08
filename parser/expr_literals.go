package parser

// GET RID OF LATER!

import (
	"gl3/lexer"
	"strconv"
	"strings"
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
	// the lexer only produces decimal and 0x literals; base 0 would also read a leading 0 as octal
	literal := p.currToken.Literal
	var uvalue uint64
	var err error
	if hex, ok := strings.CutPrefix(literal, "0x"); ok {
		uvalue, err = strconv.ParseUint(hex, 16, 64)
	} else if len(literal) > 1 && literal[0] == '0' {
		p.appendError(&p.currToken.Position, "integer literal %q has a leading zero", literal)
	} else {
		uvalue, err = strconv.ParseUint(literal, 10, 64)
	}
	if err != nil {
		p.appendError(&p.currToken.Position, "could not parse %q as integer", literal)
	}

	lit.UValue = uvalue

	switch p.currToken.Suffix {
	case "":
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
		p.appendError(&p.currToken.Position, "unknown integer literal suffix %s", p.currToken.Suffix)
	}
	p.NextToken()

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

	value, err := strconv.ParseFloat(p.currToken.Literal, 64)
	if err != nil {
		p.appendError(&p.currToken.Position, "could not parse %q as float", p.currToken.Literal)
	}

	lit.Value = value

	switch p.currToken.Suffix {
	case "", "f64":
		lit.Type.Base = lexer.Float
	case "f32":
		lit.Type.Base = lexer.Float32
	default:
		lit.Type.Base = lexer.None
		p.appendError(&p.currToken.Position, "unknown float literal suffix %s", p.currToken.Suffix)
	}

	p.NextToken()

	return lit
}

func (p *Parser) parseCharLiteral() Expression {
	if p.currToken.Literal == "" {
		p.appendError(&p.currToken.Position, "empty character literal")
		p.NextToken()
		return nil
	}
	vt := lexer.VarType{Base: lexer.Int8, Pointer: 0}
	expr := &IntegerLiteral{Token: p.currToken, UValue: uint64(p.currToken.Literal[0]), Type: vt}
	p.NextToken()
	return expr
}

func (p *Parser) parseNullptr() Expression {
	expr := &NullptrLiteral{Token: p.currToken}
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
