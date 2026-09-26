package parser

import (
	"gl3/lexer"
	"gl3/util"
)

func (p *Parser) parseInfixExpression(left Expression) Expression {
	expression := &InfixExpression{
		Token:    p.currToken,
		Operator: p.currToken.Literal,
		Left:     left,
	}

	precendece := p.currPrecedence()
	p.NextToken()
	expression.Right = p.parseExpression(precendece)

	return expression
}

func (p *Parser) parseCastExpression(left Expression) Expression {
	expr := &CastExpression{Token: p.currToken, position: util.Position{
		StartLine: p.currToken.Position.StartLine,
		StartCol:  p.currToken.Position.StartCol,
	}}
	expr.Expr = left

	p.NextToken() // asvance past AS
	castType, ok := p.parseType()
	if !ok {
		expr.Position().CopyEnd(&p.currToken.Position)
		p.appendError(&expr.position, "expected type after as token in cast expr")
		return nil
	}
	expr.Type = castType
	expr.Position().CopyEnd(&p.currToken.Position)

	return expr
}

func (p *Parser) parseAssignExpression(left Expression) Expression {
	expr := &AssignmentExpression{Token: p.currToken}
	switch left.(type) {
	case *IdentifierExpression, *DereferenceExpression:
		expr.Left = left
	default:
		if infix, ok := left.(*InfixExpression); ok && infix.Operator == "." {
			expr.Left = left
		} else {
			p.appendError(left.Position(), "got %T on lhs of assignment, expected ident or deref", left)
		}
	}
	p.NextToken()

	expr.Right = p.parseExpression(LOWEST)
	if expr.Right == nil {
		return nil
	}

	return expr
}

// parseArrayIndexExpression, this is rather dodgy as it basically attempts to be a sugar for deref + pointer arithmetic
// to keep the same semantics
func (p *Parser) parseArrayIndexExpression(left Expression) Expression {
	derefToken := p.currToken
	p.NextToken() // skip past [
	index := p.parseExpression(LOWEST)
	if !p.expectCurr(lexer.RBRACKET) {
		return nil
	}

	return &DereferenceExpression{
		Token: derefToken,
		Var:   &InfixExpression{Token: p.currToken, Left: left, Operator: "+", Right: index},
	}
}

func (p *Parser) parseCallExpression(left Expression) Expression {
	identExpr, ok := left.(*IdentifierExpression)
	if !ok {
		p.appendError(&p.currToken.Position, "only functions can be called; missing `;` before `(`?")
		return nil
	}
	leftPos := left.Position()
	exp := &CallExpression{Token: p.currToken, Function: identExpr, position: util.Position{
		StartLine: leftPos.StartLine,
		StartCol:  leftPos.StartCol,
	}}
	if !p.expectCurr(lexer.LPAREN) {
		return nil
	}

	params, ok := p.parseExpressionList(lexer.RPAREN)
	if !ok {
		exp.Position().CopyEnd(&p.currToken.Position)
		p.appendError(exp.Position(), "expected ) or , when parsing param list in call expression")
		return nil
	}
	exp.Params = params

	exp.Position().CopyEnd(&p.currToken.Position)
	p.NextToken()

	return exp
}

func (p *Parser) parseStructInitialization(left Expression) Expression {
	exp := &StructInitializationExpression{Token: p.currToken, position: util.Position{
		StartLine: p.currToken.Position.StartLine,
		StartCol:  p.currToken.Position.StartCol,
	}}
	if ident, ok := left.(*IdentifierExpression); ok {
		exp.Name = ident.Value
	} else {
		exp.position.CopyEnd(&p.currToken.Position)
		p.appendError(exp.Position(), "expected identifier on lhs of struct init")
		return nil
	}
	p.NextToken() // skip past :
	if !p.expectCurr(lexer.LBRACE) {
		p.NextToken()
	}

	values, ok := p.parseExpressionList(lexer.RBRACE)
	if !ok {
		exp.position.CopyEnd(&p.currToken.Position)
		p.appendError(exp.Position(), "expected ] or , in struct init expression")
		return nil
	}
	exp.Values = values

	p.NextToken()
	exp.position.CopyEnd(&p.currToken.Position)

	return exp
}
