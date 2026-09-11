package hir

type BaseType uint8

const (
	Invalid BaseType = iota
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
	Float
	StructType
)

type Type struct {
	Base    BaseType
	Pointer uint8
	Struct  StructID
}

type TypedName struct {
	Name string
	Type Type
}

type Struct struct {
	Name       string
	Id         StructID
	Fields     []TypedName
	FieldNames map[string]int
	Opaque     bool
	Unsized    bool // Populated by pass 1.2; includes indirect fields and by-value cycles.
	// only Opaque structs can be Private
	Private bool
}

type Function struct {
	Name           string
	Id             FunctionID
	Parameters     []TypedName
	ParameterNames map[string]int
	ReturnType     Type
	Private        bool
	// Parameters occupy the first len(Parameters) entries in Locals, in order.
	Locals   []Local
	External bool
	Body     Block
}

type Global struct {
	Name        string
	Id          GlobalID
	Constant    bool
	Type        Type
	Initializer Expr
}

type Program struct {
	Structs   []Struct
	Functions []Function
	Globals   []Global
}

type Local struct {
	Name string
	Type Type
}

type Expr interface {
	exprNode()
	Type() Type
}

type Stmt interface {
	stmtNode()
}

type ExprInfo struct {
	ResultType Type
}

func (e ExprInfo) Type() Type { return e.ResultType }

type LocalRef struct {
	ExprInfo
	ID LocalID
}

func (*LocalRef) exprNode() {}

type GlobalRef struct {
	ExprInfo
	ID GlobalID
}

func (*GlobalRef) exprNode() {}

type IntegerLiteral struct {
	ExprInfo
	// Value holds the integer bits, interpreted using ResultType's width and signedness.
	Value uint64
}

func (*IntegerLiteral) exprNode() {}

type BooleanLiteral struct {
	ExprInfo
	Value bool
}

func (*BooleanLiteral) exprNode() {}

type FloatLiteral struct {
	ExprInfo
	Value float32
}

func (*FloatLiteral) exprNode() {}

type StringLiteral struct {
	ExprInfo
	Value string
}

func (*StringLiteral) exprNode() {}

type BinaryOp uint8

const (
	InvalidBinaryOp BinaryOp = iota
	IntAdd
	IntSubtract
	IntMultiply
	SignedDivide
	UnsignedDivide
	FloatAdd
	FloatSubtract
	FloatMultiply
	FloatDivide
	IntEqual
	IntNotEqual
	SignedLess
	SignedLessEqual
	SignedGreater
	SignedGreaterEqual
	UnsignedLess
	UnsignedLessEqual
	UnsignedGreater
	UnsignedGreaterEqual
	FloatEqual
	FloatNotEqual
	FloatLess
	FloatLessEqual
	FloatGreater
	FloatGreaterEqual
	BoolAnd
	BoolOr
	PointerAdd
	PointerSubtract
)

type Binary struct {
	ExprInfo
	Op          BinaryOp
	Left, Right Expr
}

func (*Binary) exprNode() {}

type Call struct {
	ExprInfo
	Function FunctionID
	Args     []Expr
}

func (*Call) exprNode() {}

type CastKind uint8

const (
	InvalidCastKind CastKind = iota
	IdentityCast
	SignExtend
	ZeroExtend
	Truncate
	SignedIntToFloat
	UnsignedIntToFloat
	FloatToSignedInt
	FloatToUnsignedInt
	PointerToInt
	IntToPointer
	PointerCast
)

type Cast struct {
	ExprInfo
	Kind  CastKind
	Value Expr
}

func (*Cast) exprNode() {}

type Block struct {
	Statements []Stmt
}

func (*Block) stmtNode() {}

type LocalDeclaration struct {
	ID          LocalID
	Initializer Expr
}

func (*LocalDeclaration) stmtNode() {}

type Return struct {
	Value Expr // nil for a void return
}

func (*Return) stmtNode() {}

type ExpressionStatement struct {
	Expr Expr
}

func (*ExpressionStatement) stmtNode() {}

type UnaryOp uint8

const (
	InvalidUnaryOp UnaryOp = iota
	IntNegate
	FloatNegate
	BoolNot
)

type Unary struct {
	ExprInfo
	Op    UnaryOp
	Value Expr
}

func (*Unary) exprNode() {}

// Place identifies storage. Type is the stored value's type, not its address type.
type Place interface {
	placeNode()
	Type() Type
}

type LocalPlace struct {
	ExprInfo
	ID LocalID
}

func (*LocalPlace) placeNode() {}

type GlobalPlace struct {
	ExprInfo
	ID GlobalID
}

func (*GlobalPlace) placeNode() {}

type DerefPlace struct {
	ExprInfo
	Pointer Expr
}

func (*DerefPlace) placeNode() {}

type FieldPlace struct {
	ExprInfo
	Base       Place
	FieldIndex int
}

func (*FieldPlace) placeNode() {}

type Assignment struct {
	ExprInfo
	Target Place
	Value  Expr
}

func (*Assignment) exprNode() {}

type AddressOf struct {
	ExprInfo
	Target Place
}

func (*AddressOf) exprNode() {}

type Dereference struct {
	ExprInfo
	Pointer Expr
}

func (*Dereference) exprNode() {}

// Base is a struct value; pointer receivers are explicitly dereferenced first.
type FieldAccess struct {
	ExprInfo
	Base       Expr
	FieldIndex int
}

func (*FieldAccess) exprNode() {}

type StructLiteral struct {
	ExprInfo
	Fields []Expr // declaration order
}

func (*StructLiteral) exprNode() {}

type ArrayLiteral struct {
	ExprInfo    // ResultType is a pointer to ElementType.
	ElementType Type
	Items       []Expr
}

func (*ArrayLiteral) exprNode() {}

type Sizeof struct {
	ExprInfo
	OperandType Type
}

func (*Sizeof) exprNode() {}

type If struct {
	Condition Expr
	Then      Block
	Else      *Block // nil if there is no else branch
}

func (*If) stmtNode() {}

type While struct {
	Condition Expr
	Body      Block
}

func (*While) stmtNode() {}

type Break struct{}

func (*Break) stmtNode() {}

type Continue struct{}

func (*Continue) stmtNode() {}
