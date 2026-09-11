package parser

import (
	"gl3/lexer"
	"gl3/util"
)

func (p *Parser) parseIfStatement() Statement {
	stmt := &IfStatement{Token: p.currToken, position: util.Position{
		StartLine: p.currToken.Position.StartLine,
		StartCol:  p.currToken.Position.StartCol,
	}}
	p.NextToken() // past IF token
	cond := p.parseExpression(LOWEST)
	stmt.Condition = cond
	if !p.expectCurr(lexer.LBRACE) {
		return nil
	}
	stmt.Success = p.parseBlockStatement()

	stmt.Position().EndLine = p.currToken.Position.EndLine
	stmt.Position().EndCol = p.currToken.Position.EndCol

	if !p.expectCurr(lexer.RBRACE) {
		return nil
	}

	if !p.currTokenIs(lexer.ELSE) {
		return stmt
	}
	p.NextToken()
	if !p.expectCurr(lexer.LBRACE) {
		return nil
	}
	stmt.Fail = p.parseBlockStatement()

	stmt.position.CopyEnd(&p.currToken.Position)
	if !p.expectCurr(lexer.RBRACE) {
		return nil
	}

	return stmt
}

func (p *Parser) parseReturnStatement() Statement {
	stmt := &ReturnStatement{Token: p.currToken}
	p.NextToken()
	// kind of a hack? but cant think of a better way lol
	if p.currTokenIs(lexer.RBRACE) ||
		p.currTokenIs(lexer.SEMICOLON) {
		return stmt
	}
	expr := p.parseExpression(LOWEST)
	stmt.Expr = expr
	return stmt
}

func (p *Parser) parseBlockStatement() *BlockStatement {
	bs := &BlockStatement{Token: p.currToken}
	bs.Statements = []Statement{}

	for !p.currTokenIs(lexer.RBRACE) {
		stmt := p.parseStatement()
		if stmt == nil {
			break
		}
		if es, ok := stmt.(*ExpressionStatement); ok && es.Expression == nil {
			break
		}
		bs.Statements = append(bs.Statements, stmt)
		if p.currTokenIs(lexer.SEMICOLON) {
			p.NextToken()
		}
	}

	return bs
}

func (p *Parser) parseBreakStatement() Statement {
	stmt := &BreakStatement{Token: p.currToken}
	p.NextToken()

	return stmt
}

func (p *Parser) parseContinueStatement() Statement {
	stmt := &ContinueStatement{Token: p.currToken}
	p.NextToken()

	return stmt
}

func (p *Parser) parseWhileStatement() Statement {
	stmt := &WhileStatement{Token: p.currToken, position: util.Position{
		StartLine: p.currToken.Position.StartLine,
		StartCol:  p.currToken.Position.StartCol,
	}}
	p.NextToken()
	cond := p.parseExpression(LOWEST)
	stmt.Condition = cond
	if !p.expectCurr(lexer.LBRACE) {
		return nil
	}
	stmt.Body = p.parseBlockStatement()

	stmt.Position().CopyEnd(&p.currToken.Position)
	if !p.expectCurr(lexer.RBRACE) {
		return nil
	}
	return stmt
}
