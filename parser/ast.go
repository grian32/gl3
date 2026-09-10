package parser

import (
	"bytes"
	"fmt"
	"gl3/lexer"
	"gl3/util"
	"strconv"
	"strings"
)

var zeroPos = util.Position{
	StartLine: 0,
	EndLine:   0,
	StartCol:  0,
	EndCol:    0,
}

type Node interface {
	TokenLiteral() string
	String() string
	Position() *util.Position
}

type Statement interface {
	Node
	statementNode()
}

type Expression interface {
	Node
	expressionNode()
}

type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}
func (p *Program) String() string {
	var out bytes.Buffer

	for _, s := range p.Statements {
		out.WriteString(s.String())
		out.WriteString(";")
	}

	return out.String()
}
func (p *Program) Position() *util.Position {
	if len(p.Statements) == 0 {
		return &zeroPos
	}
	firstPos := p.Statements[0].Position()
	lastPos := p.Statements[len(p.Statements)-1].Position()
	return &util.Position{
		StartLine: firstPos.StartLine,
		StartCol:  firstPos.StartCol,
		EndLine:   lastPos.EndLine,
		EndCol:    lastPos.EndCol,
	}
}

type ExpressionStatement struct {
	Token      lexer.Token
	Expression Expression
}

func (es *ExpressionStatement) statementNode()       {}
func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExpressionStatement) String() string {
	if es.Expression != nil {
		return es.Expression.String()
	}

	return ""
}
func (es *ExpressionStatement) Position() *util.Position {
	if es.Expression != nil {
		return es.Expression.Position()
	}

	return &zeroPos
}

type IntegerLiteral struct {
	Token lexer.Token
	Type  lexer.VarType
	// Value get rid of once emitter is rewritten.
	Value  int64
	UValue uint64
}

func (il *IntegerLiteral) expressionNode()      { /* noop */ }
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }
func (il *IntegerLiteral) String() string {
	switch il.Type.Base {
	case lexer.Int8, lexer.Int16, lexer.Int32, lexer.Int,
		lexer.Uint8, lexer.Uint16, lexer.Uint32, lexer.Uint:
		return strconv.FormatUint(il.UValue, 10) + "(" + il.Type.String() + ")"
	}
	return il.Token.Literal + "(" + il.Type.String() + ")"
}
func (il *IntegerLiteral) Position() *util.Position {
	pos := il.Token.Position
	switch il.Type.Base {
	case lexer.Int8, lexer.Uint8:
		pos.EndCol += 2
	case lexer.Int16, lexer.Uint16, lexer.Int32, lexer.Uint32, lexer.Uint:
		pos.EndCol += 3 // account for literal
	}

	return &pos
}

type InfixExpression struct {
	Token    lexer.Token
	Left     Expression
	Operator string
	Right    Expression
}

func (ie *InfixExpression) expressionNode()      { /* noop */ }
func (ie *InfixExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *InfixExpression) String() string {
	return "(" + ie.Left.String() + " " + ie.Operator + " " + ie.Right.String() + ")"
}
func (ie *InfixExpression) Position() *util.Position {
	leftPos := ie.Left.Position()
	rightPos := ie.Right.Position()
	return &util.Position{
		StartLine: leftPos.StartLine,
		StartCol:  leftPos.StartCol,
		EndLine:   rightPos.EndLine,
		EndCol:    rightPos.EndCol,
	}
}

type DefStatement struct {
	Token    lexer.Token
	Name     *IdentifierExpression
	Type     lexer.VarType
	Right    Expression
	Global   bool
	Constant bool
}

func (ds *DefStatement) statementNode()       { /* noop */ }
func (ds *DefStatement) TokenLiteral() string { return ds.Token.Literal }
func (ds *DefStatement) String() string {
	var str strings.Builder
	if ds.Global {
		str.WriteString("global ")
		if ds.Constant {
			str.WriteString("const ")
		}
	} else {
		str.WriteString("def ")
	}

	str.WriteString(ds.Type.String())
	str.WriteRune(' ')
	str.WriteString(ds.Name.String())
	str.WriteString(" = ")
	str.WriteString(ds.Right.String())

	return str.String()
}
func (ds *DefStatement) Position() *util.Position {
	tokenPos := ds.Token.Position
	rightPos := ds.Right.Position()

	return &util.Position{
		StartLine: tokenPos.StartLine,
		StartCol:  tokenPos.StartCol,
		EndLine:   rightPos.EndLine,
		EndCol:    rightPos.EndCol,
	}
}

type AssignmentExpression struct {
	Token lexer.Token
	Left  Expression
	Right Expression
}

func (ae *AssignmentExpression) expressionNode()      { /* noop */ }
func (ae *AssignmentExpression) TokenLiteral() string { return ae.Token.Literal }
func (ae *AssignmentExpression) String() string {
	return ae.Left.String() + " = " + ae.Right.String()
}
func (ae *AssignmentExpression) Position() *util.Position {
	leftPos := ae.Left.Position()
	rightPos := ae.Right.Position()
	return &util.Position{
		StartLine: leftPos.StartLine,
		StartCol:  leftPos.StartCol,
		EndLine:   rightPos.EndLine,
		EndCol:    rightPos.EndCol,
	}
}

type IdentifierExpression struct {
	Token lexer.Token
	Value string
}

func (ie *IdentifierExpression) expressionNode()          { /* noop */ }
func (ie *IdentifierExpression) TokenLiteral() string     { return ie.Token.Literal }
func (ie *IdentifierExpression) String() string           { return ie.Value }
func (ie *IdentifierExpression) Position() *util.Position { return &ie.Token.Position }

type ReturnStatement struct {
	Token lexer.Token
	Expr  Expression
}

func (rs *ReturnStatement) statementNode()       { /* noop */ }
func (rs *ReturnStatement) TokenLiteral() string { return rs.Token.Literal }
func (rs *ReturnStatement) String() string       { return "return " + rs.Expr.String() }
func (rs *ReturnStatement) Position() *util.Position {
	tokPos := rs.Token.Position
	exprPos := rs.Expr.Position()
	return &util.Position{
		StartLine: tokPos.StartLine,
		StartCol:  tokPos.StartCol,
		EndLine:   exprPos.EndLine,
		EndCol:    exprPos.EndCol,
	}
}

type BlockStatement struct {
	Token      lexer.Token
	Statements []Statement
}

func (bs *BlockStatement) statementNode()       { /* noop */ }
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BlockStatement) String() string {
	var out bytes.Buffer

	for i, s := range bs.Statements {
		out.WriteString(s.String())
		if i != len(bs.Statements)-1 {
			out.WriteString(";")
		}
	}

	return out.String()
}

// Position should not be used
func (bs *BlockStatement) Position() *util.Position {
	return &zeroPos
}

type FunctionParameter struct {
	Type lexer.VarType
	Name *IdentifierExpression
}

func (fp *FunctionParameter) String() string { return fp.Type.String() + " " + fp.Name.String() }

type FunctionStatement struct {
	Token    lexer.Token
	Name     *IdentifierExpression
	Type     lexer.VarType
	Params   []FunctionParameter
	Body     *BlockStatement
	Private  bool
	position util.Position
}

func (fs *FunctionStatement) statementNode()       { /* noop */ }
func (fs *FunctionStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *FunctionStatement) String() string {
	var out bytes.Buffer

	if fs.Private {
		out.WriteString("private ")
	}

	out.WriteString("fnc " + fs.Name.String() + "(")

	for i, p := range fs.Params {
		out.WriteString(p.String())
		if i != len(fs.Params)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteString(") -> " + fs.Type.String() + " { " + fs.Body.String() + " }")
	return out.String()
}
func (fs *FunctionStatement) Position() *util.Position {
	return &fs.position
}

type CallExpression struct {
	Token    lexer.Token
	Function *IdentifierExpression
	Params   []Expression
	position util.Position
}

func (ce *CallExpression) expressionNode()      { /* noop */ }
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *CallExpression) String() string {
	var out bytes.Buffer
	out.WriteString(ce.Function.String() + "(")

	for i, p := range ce.Params {
		out.WriteString(p.String())
		if i != len(ce.Params)-1 {
			out.WriteString(", ")
		}
	}

	out.WriteString(")")

	return out.String()
}
func (ce *CallExpression) Position() *util.Position {
	return &ce.position
}

type ReferenceExpression struct {
	Token lexer.Token
	Var   Expression
}

func (re *ReferenceExpression) expressionNode()      { /* noop */ }
func (re *ReferenceExpression) TokenLiteral() string { return re.Token.Literal }
func (re *ReferenceExpression) String() string       { return "&" + re.Var.String() }
func (re *ReferenceExpression) Position() *util.Position {
	vPos := re.Var.Position()
	return &util.Position{
		StartLine: re.Token.Position.StartLine,
		StartCol:  re.Token.Position.StartCol,
		EndLine:   vPos.EndLine,
		EndCol:    vPos.EndCol,
	}
}

type DereferenceExpression struct {
	Token lexer.Token
	Var   Expression
}

func (de *DereferenceExpression) expressionNode()      { /* noop */ }
func (de *DereferenceExpression) TokenLiteral() string { return de.Token.Literal }
func (de *DereferenceExpression) String() string       { return "*" + de.Var.String() }
func (de *DereferenceExpression) Position() *util.Position {
	vPos := de.Var.Position()
	return &util.Position{
		StartLine: de.Token.Position.StartLine,
		StartCol:  de.Token.Position.StartCol,
		EndLine:   vPos.EndLine,
		EndCol:    vPos.EndCol,
	}
}

type BooleanExpression struct {
	Token lexer.Token
	Value bool
}

func (be *BooleanExpression) expressionNode()      { /* noop */ }
func (be *BooleanExpression) TokenLiteral() string { return be.Token.Literal }
func (be *BooleanExpression) String() string       { return fmt.Sprintf("%t", be.Value) }
func (be *BooleanExpression) Position() *util.Position {
	return &be.Token.Position
}

type PrefixExpression struct {
	Token    lexer.Token
	Operator string
	Right    Expression
}

func (pe *PrefixExpression) expressionNode()      { /* noop */ }
func (pe *PrefixExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PrefixExpression) String() string       { return "(" + pe.Operator + pe.Right.String() + ")" }
func (pe *PrefixExpression) Position() *util.Position {
	rPos := pe.Right.Position()
	return &util.Position{
		StartLine: pe.Token.Position.StartLine,
		StartCol:  pe.Token.Position.StartCol,
		EndLine:   rPos.EndLine,
		EndCol:    rPos.EndCol,
	}
}

type CastExpression struct {
	Token    lexer.Token
	Expr     Expression
	Type     lexer.VarType
	position util.Position
}

func (ce *CastExpression) expressionNode()      { /* noop */ }
func (ce *CastExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *CastExpression) String() string       { return ce.Expr.String() + " as " + ce.Type.String() }
func (ce *CastExpression) Position() *util.Position {
	return &ce.position
}

type FloatLiteral struct {
	Token lexer.Token
	Value float32
	Type  lexer.VarType // opts : Float only
}

func (fl *FloatLiteral) expressionNode()      { /* noop */ }
func (fl *FloatLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FloatLiteral) String() string       { return fl.Token.Literal + "(" + fl.Type.String() + ")" }
func (fl *FloatLiteral) Position() *util.Position {
	return &fl.Token.Position
}

type SizeofExpression struct {
	Token    lexer.Token
	Type     lexer.VarType
	position util.Position
}

func (se *SizeofExpression) expressionNode()      { /* noop */ }
func (se *SizeofExpression) TokenLiteral() string { return se.Token.Literal }
func (se *SizeofExpression) String() string       { return "sizeof " + se.Type.String() }
func (se *SizeofExpression) Position() *util.Position {
	return &se.position
}

type ArrayLiteral struct {
	Token    lexer.Token
	Type     lexer.VarType
	Items    []Expression
	position util.Position
}

func (al *ArrayLiteral) expressionNode()      { /* noop */ }
func (al *ArrayLiteral) TokenLiteral() string { return al.Token.Literal }
func (al *ArrayLiteral) String() string {
	var out bytes.Buffer
	out.WriteString("[")
	out.WriteString(al.Type.String())
	out.WriteString(";")

	for i, e := range al.Items {
		out.WriteString(e.String())
		if i != len(al.Items)-1 {
			out.WriteString(",")
		}
	}
	out.WriteString("]")

	return out.String()
}
func (al *ArrayLiteral) Position() *util.Position {
	return &al.position
}

type ImportStatement struct {
	Token    lexer.Token
	Path     string
	position util.Position
}

func (is *ImportStatement) statementNode()       { /* noop */ }
func (is *ImportStatement) TokenLiteral() string { return is.Token.Literal }
func (is *ImportStatement) String() string       { return "import \"" + is.Path + "\"" }
func (is *ImportStatement) Position() *util.Position {
	return &is.position
}

type StringLiteral struct {
	Token lexer.Token
	Value string
}

func (sl *StringLiteral) expressionNode()      { /* noop */ }
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StringLiteral) String() string       { return "\"" + sl.Value + "\"" }
func (sl *StringLiteral) Position() *util.Position {
	return &sl.Token.Position
}

type IfStatement struct {
	Token     lexer.Token
	Condition Expression
	Success   *BlockStatement
	Fail      *BlockStatement
	position  util.Position
}

func (is *IfStatement) statementNode()       { /* noop */ }
func (is *IfStatement) TokenLiteral() string { return is.Token.Literal }
func (is *IfStatement) String() string {
	var out bytes.Buffer
	out.WriteString("if ")
	out.WriteString(is.Condition.String())
	out.WriteString(" { ")
	out.WriteString(is.Success.String())
	out.WriteString(" }")
	if is.Fail != nil {
		out.WriteString(" else { ")
		out.WriteString(is.Fail.String())
		out.WriteString(" }")
	}

	return out.String()
}
func (is *IfStatement) Position() *util.Position {
	return &is.position
}

type WhileStatement struct {
	Token     lexer.Token
	Condition Expression
	Body      *BlockStatement
	position  util.Position
}

func (ws *WhileStatement) statementNode()       { /* noop */ }
func (ws *WhileStatement) TokenLiteral() string { return ws.Token.Literal }
func (ws *WhileStatement) String() string {
	var out bytes.Buffer
	out.WriteString("while ")
	out.WriteString(ws.Condition.String())
	out.WriteString(" { ")
	out.WriteString(ws.Body.String())
	out.WriteString(" }")

	return out.String()
}
func (ws *WhileStatement) Position() *util.Position {
	return &ws.position
}

type StructField struct {
	Name *IdentifierExpression
	Type lexer.VarType
}

type StructStatement struct {
	Token    lexer.Token
	Name     string
	Fields   []StructField
	position util.Position
}

func (ss *StructStatement) statementNode()       { /* noop */ }
func (ss *StructStatement) TokenLiteral() string { return ss.Token.Literal }
func (ss *StructStatement) String() string {
	var out bytes.Buffer
	out.WriteString("struct ")
	out.WriteString(ss.Name)
	out.WriteString("{")
	for _, field := range ss.Fields {
		out.WriteString(field.Type.String())
		out.WriteString(" ")
		out.WriteString(field.Name.Value)
		out.WriteString(";")
	}
	out.WriteString("}")
	return out.String()
}
func (ss *StructStatement) Position() *util.Position {
	return &ss.position
}

type StructInitializationExpression struct {
	Token    lexer.Token
	Name     string
	Values   []Expression
	position util.Position
}

func (sie *StructInitializationExpression) expressionNode()      { /* noop */ }
func (sie *StructInitializationExpression) TokenLiteral() string { return sie.Token.Literal }
func (sie *StructInitializationExpression) String() string {
	var out bytes.Buffer
	out.WriteString(sie.Name)
	out.WriteString(":{")
	for i, e := range sie.Values {
		out.WriteString(e.String())
		if i != len(sie.Values)-1 {
			out.WriteString(",")
		}
	}
	out.WriteString("}")
	return out.String()
}
func (sie *StructInitializationExpression) Position() *util.Position {
	return &sie.position
}

type BreakStatement struct {
	Token lexer.Token
}

func (b *BreakStatement) statementNode()           { /* noop */ }
func (b *BreakStatement) TokenLiteral() string     { return b.Token.Literal }
func (b *BreakStatement) String() string           { return b.Token.Literal }
func (b *BreakStatement) Position() *util.Position { return &b.Token.Position }

type ContinueStatement struct {
	Token lexer.Token
}

func (c *ContinueStatement) statementNode()           { /* noop */ }
func (c *ContinueStatement) TokenLiteral() string     { return c.Token.Literal }
func (c *ContinueStatement) String() string           { return c.Token.Literal }
func (c *ContinueStatement) Position() *util.Position { return &c.Token.Position }

type ExternStructStatement struct {
	Token    lexer.Token
	Name     string
	Private  bool
	position util.Position
}

func (e *ExternStructStatement) statementNode()       { /* noop */ }
func (e *ExternStructStatement) TokenLiteral() string { return e.Token.Literal }
func (e *ExternStructStatement) String() string {
	if e.Private {
		return "private extern struct " + e.Name
	}
	return "extern struct " + e.Name
}
func (e *ExternStructStatement) Position() *util.Position { return &e.position }

type ExternFunctionStatement struct {
	Token      lexer.Token
	Name       string
	ReturnType lexer.VarType
	Params     []FunctionParameter
	Private    bool
	position   util.Position
}

func (e *ExternFunctionStatement) statementNode()       { /* noop */ }
func (e *ExternFunctionStatement) TokenLiteral() string { return e.Token.Literal }
func (e *ExternFunctionStatement) String() string {
	var out bytes.Buffer

	if e.Private {
		out.WriteString("private ")
	}
	out.WriteString("extern fnc " + e.Name + "(")

	for i, p := range e.Params {
		out.WriteString(p.String())
		if i != len(e.Params)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteString(") -> " + e.ReturnType.String())

	return out.String()
}
func (e *ExternFunctionStatement) Position() *util.Position { return &e.position }
