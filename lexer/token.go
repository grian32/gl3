package lexer

import (
	"gl3/util"
	"strings"
)

type TokenType uint8

const (
	UNKNOWN TokenType = iota
	INT
	FLOAT
	PLUS
	MINUS
	ASTERISK
	SLASH
	PERCENT
	PIPE
	CARET
	TILDE
	SHL
	SHR
	PLUS_ASSIGN
	MINUS_ASSIGN
	ASTERISK_ASSIGN
	SLASH_ASSIGN
	PERCENT_ASSIGN
	AMPERSAND_ASSIGN
	PIPE_ASSIGN
	CARET_ASSIGN
	SHL_ASSIGN
	SHR_ASSIGN
	SEMICOLON
	ASSIGN
	IDENTIFIER
	DEF
	FNC
	LPAREN
	RPAREN
	COMMA
	LBRACE
	RBRACE
	RETURN
	TYPE
	ARROW
	AMPERSAND
	TRUE
	FALSE
	NOT
	LAND
	LOR
	EQ
	NOTEQ
	LT
	LTEQ
	GT
	GTEQ
	AS
	SIZEOF
	LBRACKET
	RBRACKET
	IMPORT
	STRING
	CHAR
	IF
	ELSE
	WHILE
	STRUCT
	DOT
	COLON
	BREAK
	CONTINUE
	GLOBAL
	CONST
	EXTERN
	PRIVATE
	NULLPTR
	ELLIPSIS
	EOF
)

func (tt TokenType) String() string {
	switch tt {
	case INT:
		return "INT"
	case PLUS:
		return "+"
	case SLASH:
		return "/"
	case PERCENT:
		return "%"
	case PIPE:
		return "|"
	case CARET:
		return "^"
	case TILDE:
		return "~"
	case SHL:
		return "<<"
	case SHR:
		return ">>"
	case PLUS_ASSIGN:
		return "+="
	case MINUS_ASSIGN:
		return "-="
	case ASTERISK_ASSIGN:
		return "*="
	case SLASH_ASSIGN:
		return "/="
	case PERCENT_ASSIGN:
		return "%="
	case AMPERSAND_ASSIGN:
		return "&="
	case PIPE_ASSIGN:
		return "|="
	case CARET_ASSIGN:
		return "^="
	case SHL_ASSIGN:
		return "<<="
	case SHR_ASSIGN:
		return ">>="
	case ASTERISK:
		return "*"
	case MINUS:
		return "-"
	case SEMICOLON:
		return ";"
	case ASSIGN:
		return "="
	case IDENTIFIER:
		return "IDENTIFIER"
	case DEF:
		return "DEF"
	case FNC:
		return "FNC"
	case LPAREN:
		return "("
	case RPAREN:
		return ")"
	case COMMA:
		return ","
	case LBRACE:
		return "{"
	case RBRACE:
		return "}"
	case RETURN:
		return "RETURN"
	case TYPE:
		return "TYPE"
	case ARROW:
		return "->"
	case AMPERSAND:
		return "&"
	case TRUE:
		return "TRUE"
	case FALSE:
		return "FALSE"
	case NOT:
		return "!"
	case LAND:
		return "&&"
	case LOR:
		return "||"
	case EQ:
		return "=="
	case NOTEQ:
		return "!="
	case LT:
		return "<"
	case LTEQ:
		return "<="
	case GT:
		return ">"
	case GTEQ:
		return ">="
	case EOF:
		return "EOF"
	case AS:
		return "AS"
	case FLOAT:
		return "FLOAT"
	case SIZEOF:
		return "SIZEOF"
	case LBRACKET:
		return "["
	case RBRACKET:
		return "]"
	case IMPORT:
		return "IMPORT"
	case STRING:
		return "STRING"
	case CHAR:
		return "CHAR"
	case IF:
		return "IF"
	case ELSE:
		return "ELSE"
	case WHILE:
		return "WHILE"
	case STRUCT:
		return "STRUCT"
	case DOT:
		return "."
	case COLON:
		return ":"
	case NULLPTR:
		return "NULLPTR"
	case ELLIPSIS:
		return "..."
	default:
		return "UNKNOWN"
	}
}

type VarType struct {
	Base    BaseVarType
	Pointer uint8
	// if true ignore base, use StructName
	IsStructType bool
	StructName   string
}

type BaseVarType uint8

const (
	None BaseVarType = iota
	Int
	Int32
	Int16
	Int8
	Char
	Uint
	Uint32
	Uint16
	Uint8
	Bool
	Void
	Float   // 64 bit aka double
	Float32 // 32 bit aka float
)

func (bvt BaseVarType) String() string {
	switch bvt {
	case None:
		return "None"
	case Void:
		return "Void"
	case Int:
		return "Int"
	case Int32:
		return "Int32"
	case Int16:
		return "Int16"
	case Int8:
		return "Int8"
	case Char:
		return "Char"
	case Uint:
		return "Uint"
	case Uint32:
		return "Uint32"
	case Uint16:
		return "Uint16"
	case Uint8:
		return "Uint8"
	case Bool:
		return "Bool"
	case Float:
		return "Float"
	case Float32:
		return "Float32"
	default:
		return "Unknown"
	}
}

func (vt VarType) String() string {
	var bvt strings.Builder
	if vt.IsStructType {
		bvt.WriteString(vt.StructName)
	} else {
		bvt.WriteString(vt.Base.String())
	}

	for _ = range vt.Pointer {
		bvt.WriteString("*")
	}

	return bvt.String()
}

type Token struct {
	Type     TokenType
	VarType  VarType
	Literal  string
	Suffix   string
	Position util.Position
}
