package parser

import (
	"gl3/lexer"
	"gl3/util"
)

const (
	_ byte = iota
	LOWEST
	ASSIGN
	LOR
	LAND
	EQUALS      // ==
	LESSGREATER // > or <
	CAST
	BOR     // |
	BXOR    // ^
	BAND    // &
	SHIFT   // << or >>
	SUM     // +
	PRODUCT // *
	PREFIX  // -X or !X
	CALL
	INDEX
)

var precedences = map[lexer.TokenType]byte{
	lexer.PLUS:      SUM,
	lexer.MINUS:     SUM,
	lexer.ASTERISK:  PRODUCT,
	lexer.SLASH:     PRODUCT,
	lexer.PERCENT:   PRODUCT,
	lexer.PIPE:      BOR,
	lexer.CARET:     BXOR,
	lexer.AMPERSAND: BAND,
	lexer.SHL:       SHIFT,
	lexer.SHR:       SHIFT,
	lexer.LPAREN:    CALL,
	lexer.DOT:       CALL, // same semantic as c
	lexer.COLON:     CALL, // same semantic as c
	lexer.ASSIGN:    ASSIGN,
	lexer.NOT:       PREFIX,
	lexer.LOR:       LOR,
	lexer.LAND:      LAND,
	lexer.EQ:        EQUALS,
	lexer.NOTEQ:     EQUALS,
	lexer.GT:        LESSGREATER,
	lexer.LT:        LESSGREATER,
	lexer.GTEQ:      LESSGREATER,
	lexer.LTEQ:      LESSGREATER,
	lexer.LBRACKET:  INDEX,
	lexer.AS:        CAST,
}

type (
	prefixParseFn func() Expression
	infixParseFn  func(Expression) Expression
)

// Parser heavily based on the grpgscript parser https://github.com/grian32/grpg/grpgscript
type Parser struct {
	lexer *lexer.Lexer

	Errors []util.PositionError

	currToken lexer.Token
	peekToken lexer.Token

	prefixParseFns map[lexer.TokenType]prefixParseFn
	infixParseFns  map[lexer.TokenType]infixParseFn
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{lexer: l}

	p.NextToken()
	p.NextToken()

	p.prefixParseFns = make(map[lexer.TokenType]prefixParseFn)
	p.prefixParseFns[lexer.INT] = p.parseIntegerLiteral
	p.prefixParseFns[lexer.FLOAT] = p.parseFloatLiteral
	p.prefixParseFns[lexer.STRING] = p.parseStringLiteral
	p.prefixParseFns[lexer.MINUS] = p.parsePrefixExpression
	p.prefixParseFns[lexer.IDENTIFIER] = p.parseIdentifier
	p.prefixParseFns[lexer.LPAREN] = p.parseGroupedExpression
	p.prefixParseFns[lexer.AMPERSAND] = p.parseReference
	p.prefixParseFns[lexer.ASTERISK] = p.parseDereference
	p.prefixParseFns[lexer.TRUE] = p.parseBoolean
	p.prefixParseFns[lexer.NULLPTR] = p.parseNullptr
	p.prefixParseFns[lexer.FALSE] = p.parseBoolean
	p.prefixParseFns[lexer.NOT] = p.parsePrefixExpression
	p.prefixParseFns[lexer.TILDE] = p.parsePrefixExpression
	p.prefixParseFns[lexer.SIZEOF] = p.parseSizeofExpression
	p.prefixParseFns[lexer.LBRACKET] = p.parseArrayLiteral
	p.prefixParseFns[lexer.CHAR] = p.parseCharLiteral

	p.infixParseFns = make(map[lexer.TokenType]infixParseFn)
	p.infixParseFns[lexer.PLUS] = p.parseInfixExpression
	p.infixParseFns[lexer.MINUS] = p.parseInfixExpression
	p.infixParseFns[lexer.SLASH] = p.parseInfixExpression
	p.infixParseFns[lexer.PERCENT] = p.parseInfixExpression
	p.infixParseFns[lexer.PIPE] = p.parseInfixExpression
	p.infixParseFns[lexer.CARET] = p.parseInfixExpression
	p.infixParseFns[lexer.AMPERSAND] = p.parseInfixExpression
	p.infixParseFns[lexer.SHL] = p.parseInfixExpression
	p.infixParseFns[lexer.SHR] = p.parseInfixExpression
	p.infixParseFns[lexer.ASTERISK] = p.parseInfixExpression
	p.infixParseFns[lexer.LAND] = p.parseInfixExpression
	p.infixParseFns[lexer.LOR] = p.parseInfixExpression
	p.infixParseFns[lexer.EQ] = p.parseInfixExpression
	p.infixParseFns[lexer.LT] = p.parseInfixExpression
	p.infixParseFns[lexer.GT] = p.parseInfixExpression
	p.infixParseFns[lexer.LTEQ] = p.parseInfixExpression
	p.infixParseFns[lexer.GTEQ] = p.parseInfixExpression
	p.infixParseFns[lexer.NOTEQ] = p.parseInfixExpression
	p.infixParseFns[lexer.DOT] = p.parseInfixExpression
	p.infixParseFns[lexer.LPAREN] = p.parseCallExpression
	p.infixParseFns[lexer.ASSIGN] = p.parseAssignExpression
	p.infixParseFns[lexer.AS] = p.parseCastExpression
	p.infixParseFns[lexer.LBRACKET] = p.parseArrayIndexExpression
	p.infixParseFns[lexer.COLON] = p.parseStructInitialization

	return p
}

func (p *Parser) NextToken() {
	p.currToken = p.peekToken
	p.peekToken = p.lexer.NextToken()
}

func (p *Parser) ParseProgram() *Program {
	program := &Program{}
	program.Statements = []Statement{}

	for !p.currTokenIs(lexer.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
	}

	return program
}
func (p *Parser) parseStatement() Statement {
	switch p.currToken.Type {
	case lexer.DEF, lexer.GLOBAL:
		return p.parseVarStatement()
	case lexer.RETURN:
		return p.parseReturnStatement()
	case lexer.FNC:
		return p.parseFunctionStatement(false)
	case lexer.IMPORT:
		return p.parseImportStatement()
	case lexer.IF:
		return p.parseIfStatement()
	case lexer.WHILE:
		return p.parseWhileStatement()
	case lexer.STRUCT:
		return p.parseStructStatement()
	case lexer.BREAK:
		return p.parseBreakStatement()
	case lexer.CONTINUE:
		return p.parseContinueStatement()
	case lexer.EXTERN:
		return p.parseExternStatement(false)
	case lexer.PRIVATE:
		p.NextToken()
		switch p.currToken.Type {
		case lexer.FNC:
			return p.parseFunctionStatement(true)
		case lexer.EXTERN:
			return p.parseExternStatement(true)
		}
	}

	return p.parseExpressionStatement()
}

func (p *Parser) parseExpressionStatement() Statement {
	stmt := &ExpressionStatement{Token: p.currToken}

	stmt.Expression = p.parseExpression(LOWEST)

	if p.currTokenIs(lexer.SEMICOLON) {
		p.NextToken()
	}

	return stmt
}

func (p *Parser) parseExpression(precendence byte) Expression {
	prefix := p.prefixParseFns[p.currToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.currToken, &p.currToken.Position)
		return nil
	}
	leftExp := prefix()

	for !p.peekTokenIs(lexer.SEMICOLON) && precendence < p.currPrecedence() {
		infix := p.infixParseFns[p.currToken.Type]
		if infix == nil {
			return leftExp
		}

		//if p.currTokenIs(lexer.SEMICOLON) {
		//	p.NextToken()
		//}

		leftExp = infix(leftExp)
		if leftExp == nil {
			return nil
		}
	}

	return leftExp
}
