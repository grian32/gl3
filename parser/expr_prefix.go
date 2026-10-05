package parser

import (
	"gl3/util"
)

func (p *Parser) parsePrefixExpression() Expression {
	expression := &PrefixExpression{
		Token:    p.currToken,
		Operator: p.currToken.Literal,
	}

	p.NextToken()

	expression.Right = p.parseExpression(PREFIX)

	return expression
}

func (p *Parser) parseReference() Expression {
	expr := &ReferenceExpression{Token: p.currToken}
	p.NextToken()
	expr.Var = p.parseExpression(PREFIX)
	if expr.Var == nil {
		return nil
	}
	return expr
}

func (p *Parser) parseDereference() Expression {
	expr := &DereferenceExpression{Token: p.currToken}

	p.NextToken()
	expr.Var = p.parseExpression(PREFIX)

	return expr
}

func (p *Parser) parseSizeofExpression() Expression {
	expr := &SizeofExpression{Token: p.currToken, position: util.Position{
		StartLine: p.currToken.Position.StartLine,
		StartCol:  p.currToken.Position.StartCol,
	}}
	p.NextToken() // past sizeof
	vt, ok := p.parseType()
	if !ok {
		expr.Position().CopyEnd(&p.currToken.Position)
		p.appendError(&expr.position, "expected type after sizeof keyword")
		return nil
	}

	expr.Type = vt
	expr.Position().CopyEnd(&p.currToken.Position)

	return expr
}

func (p *Parser) parseVarargExpression() Expression {
	expr := &VarargExpression{Token: p.currToken, position: util.Position{
		StartLine: p.currToken.Position.StartLine,
		StartCol:  p.currToken.Position.StartCol,
	}}
	p.NextToken() // past vararg
	vt, ok := p.parseType()
	if !ok {
		expr.Position().CopyEnd(&p.currToken.Position)
		p.appendError(&expr.position, "expected type after vararg keyword")
		return nil
	}

	expr.Type = vt
	expr.Position().CopyEnd(&p.currToken.Position)

	return expr
}
