package emitter

import (
	"fmt"
	"gl3/lexer"
	"gl3/parser"
	"gl3/util"
	"os"
	"strings"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

type Emitter struct {
	m         *ir.Module
	currBlock *ir.Block
	currFnc   *ir.Func

	// var, vartypes, params get reset after each function is emitted.
	variables         map[string]*ir.InstAlloca
	globals           map[string]*ir.Global
	varTypes          map[string]types.Type
	varGlTypes        map[string]lexer.VarType
	parameters        map[string]*ir.Param
	parametersGlTypes map[string]lexer.VarType

	functions             map[string]*ir.Func
	functionGlReturnTypes map[string]lexer.VarType

	stringLiterals map[string]*ir.Global

	builtinModules []string

	asmModuleImported  bool
	ioModuleImported   bool
	emittingVarargArgs bool
	astFuncs           map[string]struct{}

	structTypes         map[string]*types.StructType
	structMemberIndexes map[string]map[string]int
	structMemberTypes   map[string][]lexer.VarType

	whileStack []WhileLoopState

	Errors []util.PositionError
}

// VariableState used simply for transport when restoring state across if stmt blocks
type VariableState struct {
	variables  map[string]*ir.InstAlloca
	varTypes   map[string]types.Type
	varGlTypes map[string]lexer.VarType
}

type WhileLoopState struct {
	condBlock *ir.Block
	endBlock  *ir.Block
}

func New() *Emitter {
	e := &Emitter{m: ir.NewModule()}
	e.globals = make(map[string]*ir.Global)
	e.variables = make(map[string]*ir.InstAlloca)
	e.varTypes = make(map[string]types.Type)
	e.functions = make(map[string]*ir.Func)
	e.parameters = make(map[string]*ir.Param)
	e.functionGlReturnTypes = make(map[string]lexer.VarType)
	e.parametersGlTypes = make(map[string]lexer.VarType)
	e.varGlTypes = make(map[string]lexer.VarType)
	e.stringLiterals = make(map[string]*ir.Global)
	e.asmModuleImported = false
	e.astFuncs = map[string]struct{}{
		"__asm__salloc": {},
	}
	e.structTypes = make(map[string]*types.StructType)
	e.structMemberIndexes = make(map[string]map[string]int)
	e.structMemberTypes = make(map[string][]lexer.VarType)

	//fnc = e.m.NewFunc("malloc", types.I32Ptr, ir.NewParam("val", types.I64Ptr))
	//e.functions["malloc"] = fnc
	return e
}

func (e *Emitter) Module() *ir.Module {
	return e.m
}

func (e *Emitter) BuiltinModules() []string {
	return e.builtinModules
}

// TODO: maybe look into ditching this, serves its purposes, but feels a bit wasteful
var infixIntOpTypes = map[lexer.BaseVarType]struct{}{
	lexer.Int8:   {},
	lexer.Int16:  {},
	lexer.Int32:  {},
	lexer.Int:    {},
	lexer.Uint8:  {},
	lexer.Uint16: {},
	lexer.Uint32: {},
	lexer.Uint:   {},
	lexer.Char:   {},
}

var llvmIntTypes = map[types.Type]struct{}{
	types.I1:  {},
	types.I8:  {},
	types.I16: {},
	types.I32: {},
	types.I64: {},
}

var varTypeIntTypes = map[lexer.BaseVarType]struct{}{
	lexer.Bool:   {},
	lexer.Int8:   {},
	lexer.Int16:  {},
	lexer.Int32:  {},
	lexer.Int:    {},
	lexer.Uint8:  {},
	lexer.Uint16: {},
	lexer.Uint32: {},
	lexer.Uint:   {},
}

var glTypeSInts = map[lexer.BaseVarType]struct{}{
	lexer.Int8:  {},
	lexer.Int16: {},
	lexer.Int32: {},
	lexer.Int:   {},
}

var glTypeUInts = map[lexer.BaseVarType]struct{}{
	lexer.Uint8:  {},
	lexer.Uint16: {},
	lexer.Uint32: {},
	lexer.Uint:   {},
}

func (e *Emitter) Emit(node parser.Node) (value.Value, lexer.VarType) {
	switch node := node.(type) {
	case *parser.Program:
		var last value.Value
		var lastType lexer.VarType
		for _, s := range node.Statements {
			last, lastType = e.Emit(s)
		}

		return last, lastType
	case *parser.ExpressionStatement:
		return e.Emit(node.Expression)
	case *parser.IntegerLiteral:
		/**
		fnc main() -> int32 {
			    def uint x = 18446744073709551616u64;
			    dbg_u64(x);
			    return 0i32;
			}
		NOTE: vaguely strange behaviour on such input, seems to always trunc to -1 even if ...17 or ...18 so on?
		maybe llvm quirk, not sure, don't think there's much more i can do as far as my handling of ints goes, not like
		go has uint128
		*/
		if _, ok := glTypeSInts[node.Type.Base]; ok {
			return constant.NewInt(e.varTypeToLlvm(node.Type).(*types.IntType), node.Value), node.Type
		} else if _, ok := glTypeUInts[node.Type.Base]; ok {
			return constant.NewInt(e.varTypeToLlvm(node.Type).(*types.IntType), int64(node.UValue)), node.Type
		}
	case *parser.BooleanExpression:
		// if e.emittingVarargArgs {
		// 	return constant.NewInt(types.I32, boolToI1(node.Value)), lexer.VarType{Base: lexer.Int32, Pointer: 0}
		// }
		return constant.NewInt(types.I1, boolToI1(node.Value)), lexer.VarType{Base: lexer.Bool, Pointer: 0}
	case *parser.FloatLiteral:
		return constant.NewFloat(types.Float, float64(node.Value)), lexer.VarType{Base: lexer.Float, Pointer: 0}
	case *parser.InfixExpression:
		left, leftVt := e.Emit(node.Left)
		if node.Operator == "." {
			// TODO: gep if ptr lhs
			if !leftVt.IsStructType {
				e.appendError(node.Position(), "non struct type %v on lhs of dot operator", leftVt)
			}
			var fieldName string
			if ident, ok := node.Right.(*parser.IdentifierExpression); ok {
				fieldName = ident.Value
			} else {
				e.appendError(node.Position(), "non identifier %T on rhs of dot operator", node.Right)
			}

			structType, ok := e.structTypes[leftVt.StructName]
			if !ok {
				e.appendError(node.Position(), "couldn't find struct type %s in field access", leftVt.StructName)
			}
			fieldIndexes, _ := e.structMemberIndexes[leftVt.StructName]
			fieldIndex, ok := fieldIndexes[fieldName]
			if !ok {
				e.appendError(node.Position(), "couldn't find field %s on struct of type %s", fieldName, leftVt.StructName)
			}
			fieldType := e.structMemberTypes[leftVt.StructName][fieldIndex]
			if leftVt.Pointer > 0 {
				zero := constant.NewInt(types.I32, 0)
				fieldIdxConst := constant.NewInt(types.I32, int64(fieldIndex))
				gep := e.currBlock.NewGetElementPtr(structType, left, zero, fieldIdxConst)
				return e.currBlock.NewLoad(e.varTypeToLlvm(fieldType), gep), fieldType
			} else {
				return e.currBlock.NewExtractValue(left, uint64(fieldIndex)), fieldType
			}
		}
		right, rightVt := e.Emit(node.Right)

		leftType := left.Type()
		rightType := right.Type()

		_, leftIntBaseOk := infixIntOpTypes[leftVt.Base]
		_, rightIntBaseOk := infixIntOpTypes[rightVt.Base]
		leftIntOk := leftIntBaseOk && leftVt.Pointer == 0
		rightIntOk := rightIntBaseOk && rightVt.Pointer == 0

		// TODO: extend when doubles etc
		_, leftFloatOk := leftType.(*types.FloatType)
		_, rightFloatOk := rightType.(*types.FloatType)

		if ptr, ok := leftType.(*types.PointerType); ok && rightIntOk {
			if node.Operator == "+" {
				return e.currBlock.NewGetElementPtr(ptr.ElemType, left, right), leftVt
			} else if node.Operator == "-" {
				intType := rightType.(*types.IntType)
				zero := constant.NewInt(intType, 0)
				negRight := e.currBlock.NewSub(zero, right)
				return e.currBlock.NewGetElementPtr(ptr.ElemType, left, negRight), leftVt
			}
		}

		// TODO: dodgy int8/char hack, improve soon
		if leftIntOk && rightIntOk && ((leftVt == rightVt) || (leftVt.Base == lexer.Char && rightVt.Base == lexer.Int8)) {
			switch node.Operator {
			case "+":
				return e.currBlock.NewAdd(left, right), leftVt
			case "-":
				return e.currBlock.NewSub(left, right), leftVt
			case "*":
				return e.currBlock.NewMul(left, right), leftVt
			case "==":
				return e.currBlock.NewICmp(enum.IPredEQ, left, right), leftVt
			case "!=":
				return e.currBlock.NewICmp(enum.IPredNE, left, right), leftVt
			}

			if _, ok := glTypeUInts[leftVt.Base]; ok {
				switch node.Operator {
				case "/":
					// TODO: def behaviour for /0, intmin/-1
					return e.currBlock.NewUDiv(left, right), leftVt
				case "<":
					return e.currBlock.NewICmp(enum.IPredULT, left, right), leftVt
				case ">":
					return e.currBlock.NewICmp(enum.IPredUGT, left, right), leftVt
				case "<=":
					return e.currBlock.NewICmp(enum.IPredULE, left, right), leftVt
				case ">=":
					return e.currBlock.NewICmp(enum.IPredUGE, left, right), leftVt
				}
			} else if _, ok := glTypeSInts[leftVt.Base]; ok {
				switch node.Operator {
				case "/":
					// TODO: def behaviour for /0, intmin/-1
					return e.currBlock.NewSDiv(left, right), leftVt
				case "<":
					return e.currBlock.NewICmp(enum.IPredSLT, left, right), leftVt
				case ">":
					return e.currBlock.NewICmp(enum.IPredSGT, left, right), leftVt
				case "<=":
					return e.currBlock.NewICmp(enum.IPredSLE, left, right), leftVt
				case ">=":
					return e.currBlock.NewICmp(enum.IPredSGE, left, right), leftVt
				}
			}
		}

		if leftFloatOk && rightFloatOk {
			switch node.Operator {
			case "+":
				return e.currBlock.NewFAdd(left, right), leftVt
			case "-":
				return e.currBlock.NewFSub(left, right), leftVt
			case "*":
				return e.currBlock.NewFMul(left, right), leftVt
			case "/":
				// TODO: def behaviour for /0, intmin/-1
				return e.currBlock.NewFDiv(left, right), leftVt
			case "<":
				return e.currBlock.NewFCmp(enum.FPredOLT, left, right), leftVt
			case ">":
				return e.currBlock.NewFCmp(enum.FPredOGT, left, right), leftVt
			case "<=":
				return e.currBlock.NewFCmp(enum.FPredOLE, left, right), leftVt
			case ">=":
				return e.currBlock.NewFCmp(enum.FPredOGE, left, right), leftVt
			case "==":
				return e.currBlock.NewFCmp(enum.FPredOEQ, left, right), leftVt
			case "!=":
				return e.currBlock.NewFCmp(enum.FPredONE, left, right), leftVt
			}
		}

		if leftType == types.I1 && rightType == types.I1 {
			switch node.Operator {
			case "&&":
				return e.currBlock.NewAnd(left, right), leftVt
			case "||":
				return e.currBlock.NewOr(left, right), leftVt
			}
		}

		e.appendError(node.Position(), "operator %s invalid for types %s(%s), %s(%s)", node.Operator, leftVt, node.Left, rightVt, node.Right)
	case *parser.PrefixExpression:
		switch node.Operator {
		case "!":
			right, rt := e.Emit(node.Right)
			// TODO: check right == i1
			trueVal := constant.NewInt(types.I1, 1)
			return e.currBlock.NewXor(right, trueVal), rt
		case "-":
			right, rt := e.Emit(node.Right)
			_, rightIntOk := glTypeSInts[rt.Base]
			_, rightFloatOk := right.Type().(*types.FloatType)
			if rightIntOk {
				zero := constant.NewInt(right.Type().(*types.IntType), 0)
				return e.currBlock.NewSub(zero, right), rt
			} else if rightFloatOk {
				zero := constant.NewFloat(types.Float, 0)
				return e.currBlock.NewFSub(zero, right), rt
			}
		}
	case *parser.DefStatement:
		lt := e.varTypeToLlvm(node.Type)
		right, vt := e.Emit(node.Right)

		if node.Global {
			vPtr := e.m.NewGlobal(node.Name.Value, lt)

			if _, ok := right.(constant.Constant); !ok {
				e.appendError(node.Position(), "global variable must be initialized with a constant value")
			}
			vPtr.Init = right.(constant.Constant)
			vPtr.Immutable = node.Constant
			e.globals[node.Name.Value] = vPtr
			e.varTypes[node.Name.Value] = lt
			e.varGlTypes[node.Name.Value] = vt

			return right, vt
		}

		vPtr := e.currBlock.NewAlloca(lt)
		e.variables[node.Name.Value] = vPtr
		e.varTypes[node.Name.Value] = lt
		e.varGlTypes[node.Name.Value] = vt
		e.currBlock.NewStore(right, vPtr)
		return right, vt
	case *parser.AssignmentExpression:
		if ident, ok := node.Left.(*parser.IdentifierExpression); ok {
			right, vt := e.Emit(node.Right)
			if global, ok := e.globals[ident.Value]; ok {
				e.currBlock.NewStore(right, global)
				return right, vt
			}

			vPtr, ok := e.variables[ident.Value]
			if !ok {
				e.appendError(node.Position(), "couldn't find variable of name %s used in var assignment", ident.Value)
			}
			e.currBlock.NewStore(right, vPtr)
			return right, vt
		} else if _, ok := node.Left.(*parser.DereferenceExpression); ok {
			ptr, _ := e.emitAddress(node.Left)
			right, vt := e.Emit(node.Right)
			e.currBlock.NewStore(right, ptr)
			return right, vt
		} else if infix, ok := node.Left.(*parser.InfixExpression); ok && infix.Operator == "." {
			var name string
			if ident, ok := infix.Left.(*parser.IdentifierExpression); ok {
				name = ident.Value
			} else {
				e.appendError(node.Position(), "expected identifier on lhs of dot operator")
			}
			left, leftVt := e.Emit(infix.Left)
			right, _ := e.Emit(node.Right)
			structType, ok := e.structTypes[leftVt.StructName]
			if !ok {
				e.appendError(node.Position(), "could not find struct with type %s", leftVt.StructName)
			}
			var fieldName string
			if ident, ok := infix.Right.(*parser.IdentifierExpression); ok {
				fieldName = ident.Value
			}
			fieldIdx, ok := e.structMemberIndexes[leftVt.StructName][fieldName]
			if !ok {
				e.appendError(node.Position(), "couldn't find field %s on struct of type %s", fieldName, leftVt.StructName)
			}
			if leftVt.Pointer > 0 {
				zero := constant.NewInt(types.I32, 0)
				fieldIdxConst := constant.NewInt(types.I32, int64(fieldIdx))
				gep := e.currBlock.NewGetElementPtr(structType, left, zero, fieldIdxConst)
				e.currBlock.NewStore(right, gep)
				return gep, leftVt
			} else {
				insert := e.currBlock.NewInsertValue(left, right, uint64(fieldIdx))
				vPtr, ok := e.variables[name]
				if !ok {
					e.appendError(node.Position(), "could not find variable with name %s", name)
				}
				e.currBlock.NewStore(insert, vPtr)
				return insert, leftVt
			}
		}
	case *parser.IdentifierExpression:
		if param, ok := e.parameters[node.Value]; ok {
			// if e.emittingVarargArgs && param.Typ == types.I1 {
			// 	return e.currBlock.NewZExt(param, types.I32), lexer.VarType{Base: lexer.Int32, Pointer: 0}
			// }
			return param, e.parametersGlTypes[node.Value]
		}

		if global, ok := e.globals[node.Value]; ok {
			return e.currBlock.NewLoad(e.varTypes[node.Value], global), e.varGlTypes[node.Value]
		}

		vPtr, ok := e.variables[node.Value]
		if !ok {
			e.appendError(node.Position(), "couldn't find variable of name %s used in var ref", node.Value)
		}
		vType, ok := e.varTypes[node.Value]
		if !ok {
			e.appendError(node.Position(), "couldn't find variable type of name %s used in var ref", node.Value)
		}
		load := e.currBlock.NewLoad(vType, vPtr)
		// if e.emittingVarargArgs && vType == types.I1 {
		// 	return e.currBlock.NewZExt(load, types.I32), lexer.VarType{Base: lexer.Int32, Pointer: 0}
		// }
		return load, e.varGlTypes[node.Value]
	case *parser.CallExpression:
		if _, ok := e.astFuncs[node.Function.Value]; ok && e.asmModuleImported {
			// NOTE: maybe pass node directly to emitAsmIntrinsic ? computing .Position() when it might not be used seems wasteful
			return e.emitAsmIntrinsic(node.Position(), node.Function.Value, node.Params)
		}
		var args []value.Value

		if e.ioModuleImported && node.Function.Value == "print" {
			e.emittingVarargArgs = true
		}

		for _, a := range node.Params {
			val, _ := e.Emit(a)
			args = append(args, val)
		}
		e.emittingVarargArgs = false

		fncPtr, ok := e.functions[node.Function.Value]
		e.emittingVarargArgs = false
		if !ok {
			e.appendError(node.Position(), "couldn't find function with name %s", node.Function.Value)
		}

		return e.currBlock.NewCall(fncPtr, args...), e.functionGlReturnTypes[node.Function.Value]
	case *parser.FunctionStatement:
		retType := e.varTypeToLlvm(node.Type)
		var paramTypes []*ir.Param

		for _, p := range node.Params {
			irParam := ir.NewParam(
				p.Name.Value,
				e.varTypeToLlvm(p.Type),
			)
			e.parameters[p.Name.Value] = irParam
			e.parametersGlTypes[p.Name.Value] = p.Type
			paramTypes = append(paramTypes, irParam)
		}

		fncPtr := e.m.NewFunc(node.Name.Value, retType, paramTypes...)
		e.functions[node.Name.Value] = fncPtr
		e.currBlock = fncPtr.NewBlock("")
		e.currFnc = fncPtr
		e.functionGlReturnTypes[node.Name.Value] = node.Type

		foundRet := false

		for _, s := range node.Body.Statements {
			// ehh??? maybe need to check if ret type none ? this allows an implicit return which i ... dislike
			if _, ok := s.(*parser.ReturnStatement); !foundRet && ok {
				foundRet = true
			}
			e.Emit(s)
		}

		if !foundRet {
			if node.Type.Base == lexer.Void && !node.Type.IsStructType && node.Type.Pointer == 0 {
				e.currBlock.NewRet(nil)
			} else {
				e.appendError(node.Position(), "missing return statement in non-void function of type %s", node.Type)
			}
		}

		e.parameters = make(map[string]*ir.Param)
		e.parametersGlTypes = make(map[string]lexer.VarType)
		e.variables = make(map[string]*ir.InstAlloca)
		e.varTypes = make(map[string]types.Type)
		e.varGlTypes = make(map[string]lexer.VarType)
		return fncPtr, node.Type
	case *parser.ReturnStatement:
		val, vt := e.Emit(node.Expr)
		e.currBlock.NewRet(val)
		return val, vt
	case *parser.ReferenceExpression:
		// TODO: Support general addressable expressions during the emitter rework.
		// The parser accepts expression operands; emission only supports identifiers for now.
		ident, ok := node.Var.(*parser.IdentifierExpression)
		if !ok {
			e.appendError(node.Position(), "address-of currently only supports identifiers")
			return nil, lexer.VarType{}
		}
		vPtr, ok := e.variables[ident.Value]
		if !ok {
			e.appendError(node.Position(), "couldn't find variable with name %s in reference expr", ident.Value)
		}
		t := e.varGlTypes[ident.Value]
		t.Pointer++
		return vPtr, t
	case *parser.DereferenceExpression:
		ptr, vt := e.Emit(node.Var)

		ptrTy, ok := ptr.Type().(*types.PointerType)
		if !ok {
			e.appendError(node.Position(), "cannot deref non-ptr type %v", ptrTy)
		}
		vt.Pointer--

		return e.currBlock.NewLoad(ptrTy.ElemType, ptr), vt
	case *parser.CastExpression:
		src, lt := e.Emit(node.Expr)
		if (lt.IsStructType && lt.Pointer == 0) || (node.Type.IsStructType && node.Type.Pointer == 0) {
			e.appendError(node.Position(), "casts using struct types are disallowed")
		}
		srcType := src.Type()
		dstType := e.varTypeToLlvm(node.Type)

		// TODO: could be faster if bare bool comparison? prob
		_, leftIntOk := llvmIntTypes[srcType]
		_, rightIntOk := varTypeIntTypes[node.Type.Base]

		_, leftFloatOk := src.Type().(*types.FloatType)
		rightFloatOk := node.Type.Base == lexer.Float && node.Type.Pointer == 0

		_, leftPtrOk := src.Type().(*types.PointerType)
		rightPtrOk := node.Type.Pointer > 0

		if leftIntOk && rightIntOk && node.Type.Pointer == 0 {
			if srcType == types.I1 {
				return e.currBlock.NewZExt(src, dstType), node.Type
			}

			dstSize := e.getSizeForVarType(node.Type)
			srcSize := getSizeForLlvmType(src.Type())

			if srcSize < dstSize {
				return e.currBlock.NewSExt(src, dstType), node.Type
			} else if srcSize > dstSize {
				return e.currBlock.NewTrunc(src, dstType), node.Type
			} else {
				// same size, no cast necessary
				return src, lt
			}
		} else if leftIntOk && node.Type.Pointer > 0 {
			return e.currBlock.NewIntToPtr(src, dstType), node.Type
		} else if _, ok := src.Type().(*types.PointerType); ok && rightIntOk && node.Type.Pointer == 0 {
			if node.Type.Base != lexer.Int {
				// non 64 bit which is llvm default on most (i.e 64bit) systems
				e.appendError(node.Position(), "compile warning: pointer to int cast may truncate")
			}
			return e.currBlock.NewPtrToInt(src, e.varTypeToLlvm(node.Type)), node.Type
		} else if leftIntOk && rightFloatOk {
			return e.currBlock.NewSIToFP(src, dstType), node.Type
		} else if leftFloatOk && rightIntOk {
			return e.currBlock.NewFPToSI(src, dstType), node.Type
		} else if leftPtrOk && rightPtrOk {
			/** c bitcast behaviour
			  int x = 1073741941; // 0100 0000 0000 0000 0000 0000 0111 0101 = 1073741941; as float = 2.somethingsomething
			  int* xx = &x;
			  float* fx = (float*)xx;
			  printf("%.100f", *fx); == 2.somethingsomething
			  return 0;
			*/
			return e.currBlock.NewBitCast(src, dstType), node.Type
		}
	case *parser.SizeofExpression:
		return constant.NewInt(types.I64, e.getSizeForVarType(node.Type)), lexer.VarType{Base: lexer.Uint, Pointer: 0}
	case *parser.ArrayLiteral:
		newFnc, ok := e.functions["arr_new"]
		if !ok {
			e.appendError(node.Position(), "cannot find arr_new while emitting array literal")
		}
		push, ok := e.functions["arr_push"]
		if !ok {
			e.appendError(node.Position(), "cannot find arr_push while emitting array literal")
		}

		sizeInt := constant.NewInt(types.I64, e.getSizeForVarType(node.Type))
		newCall := e.currBlock.NewCall(newFnc, sizeInt)
		node.Type.Pointer++
		newCallCasted := e.currBlock.NewBitCast(newCall, e.varTypeToLlvm(node.Type))
		ptr := e.currBlock.NewAlloca(e.varTypeToLlvm(node.Type))
		e.currBlock.NewStore(newCallCasted, ptr)
		for _, elem := range node.Items {
			v, _ := e.Emit(elem)
			e.currBlock.NewCall(push, ptr, v)
		}

		// possibly dangerous?
		return newCallCasted, node.Type
	case *parser.ImportStatement:
		if strings.HasSuffix(node.Path, ".gl3") {
			f, err := os.ReadFile(node.Path)
			if err != nil {
				e.appendError(node.Position(), "cannot find %s file described in import stmt", f)
				return nil, lexer.VarType{}
			}
			ip := findDeclares(string(f))

			for _, d := range ip.globalDefns {
				lt := e.varTypeToLlvm(d.Type)

				vPtr := &ir.Global{
					GlobalIdent: ir.GlobalIdent{
						GlobalName: d.Name,
					},
					ContentType: lt,
					Immutable:   true,
					Linkage:     enum.LinkageExternal,
				}
				e.m.Globals = append(e.m.Globals, vPtr)
				e.globals[d.Name] = vPtr
				e.varTypes[d.Name] = lt
				e.varGlTypes[d.Name] = d.Type
			}

			for _, d := range ip.structDefns {
				typ := &types.StructType{
					TypeName: d.Name,
				}
				for _, t := range d.Fields {
					typ.Fields = append(typ.Fields, e.varTypeToLlvmStructDefn(t, d.Name))
				}
				e.structTypes[d.Name] = typ
				e.structMemberIndexes[d.Name] = d.FieldNames
				e.structMemberTypes[d.Name] = d.Fields
				e.m.NewTypeDef(d.Name, typ)
			}

			for _, d := range ip.externStructDefns {
				// TODO: abstract this from emit extern struct stmt
				typ := &types.StructType{TypeName: d, Opaque: true}
				e.structTypes[d] = typ
				e.m.NewTypeDef(d, typ)
			}

			for _, d := range ip.declares {
				var params []*ir.Param
				for _, p := range d.ParamTypes {
					params = append(params, ir.NewParam("", e.varTypeToLlvm(p)))
				}
				fnc := e.m.NewFunc(d.Name, e.varTypeToLlvm(d.ReturnType), params...)
				e.functions[d.Name] = fnc
				e.functionGlReturnTypes[d.Name] = d.ReturnType
			}

		} else if node.Path == "asm" {
			e.asmModuleImported = true
		} else {
			if node.Path == "io" {
				e.ioModuleImported = true
			}
			err := AddBuiltinModule(e, node.Path)
			if err != nil {
				e.appendError(node.Position(), "couldn't import builtin module %s", node.Path)
			}
		}
	case *parser.StringLiteral:
		sVt := lexer.VarType{Base: lexer.Char, Pointer: 1}
		zero := constant.NewInt(types.I64, 0)
		if sPtr, ok := e.stringLiterals[node.Value]; ok {
			return constant.NewGetElementPtr(sPtr.ContentType, sPtr, zero, zero), sVt
		}
		str := e.m.NewGlobalDef("", constant.NewCharArrayFromString(node.Value))
		str.Linkage = enum.LinkagePrivate
		str.UnnamedAddr = enum.UnnamedAddrUnnamedAddr

		e.stringLiterals[node.Value] = str

		return constant.NewGetElementPtr(str.ContentType, str, zero, zero), sVt
	case *parser.IfStatement:
		cond, _ := e.Emit(node.Condition)
		thenBlock := e.currFnc.NewBlock("")
		endBlock := e.currFnc.NewBlock("")

		if node.Fail != nil {
			elseBlock := e.currFnc.NewBlock("")
			e.currBlock.NewCondBr(cond, thenBlock, elseBlock)

			e.currBlock = thenBlock
			saved := e.saveVariableState()
			if !e.emitBlockFindRet(node.Success) {
				e.currBlock.NewBr(endBlock)
			}
			e.loadVariableState(saved)
			e.currBlock = elseBlock

			saved = e.saveVariableState()
			if !e.emitBlockFindRet(node.Fail) {
				e.currBlock.NewBr(endBlock)
			}
			e.currBlock = endBlock
			e.loadVariableState(saved)
		} else {
			e.currBlock.NewCondBr(cond, thenBlock, endBlock)
			e.currBlock = thenBlock

			saved := e.saveVariableState()
			if !e.emitBlockFindRet(node.Success) {
				e.currBlock.NewBr(endBlock)
			}
			e.currBlock = endBlock
			e.loadVariableState(saved)
		}
	case *parser.WhileStatement:
		condBlock := e.currFnc.NewBlock("")
		whileBlock := e.currFnc.NewBlock("")
		endBlock := e.currFnc.NewBlock("")

		// emit branch to cond block
		e.currBlock.NewBr(condBlock)

		// emit condition block -- shouldnt need context saving since you cant set variables in boolean expr
		// needs to be checked at compile time
		e.currBlock = condBlock
		cond, _ := e.Emit(node.Condition)
		e.currBlock.NewCondBr(cond, whileBlock, endBlock)

		// emit while block
		saved := e.saveVariableState()
		e.whileStack = append(e.whileStack, WhileLoopState{condBlock: condBlock, endBlock: endBlock})
		e.currBlock = whileBlock
		for _, s := range node.Body.Statements {
			e.Emit(s)
			if e.currBlock.Term != nil {
				break
			}
		}
		if e.currBlock.Term == nil {
			e.currBlock.NewBr(condBlock)
		}
		e.loadVariableState(saved)
		e.whileStack = e.whileStack[:len(e.whileStack)-1]

		e.currBlock = endBlock
	case *parser.BreakStatement:
		if len(e.whileStack) == 0 {
			e.appendError(node.Position(), "break statement not allowed outside of while loop")
			return nil, lexer.VarType{}
		}

		e.currBlock.NewBr(e.whileStack[len(e.whileStack)-1].endBlock)
		return nil, lexer.VarType{}
	case *parser.ContinueStatement:
		if len(e.whileStack) == 0 {
			e.appendError(node.Position(), "continue statement not allowed outside of while loop")
			return nil, lexer.VarType{}
		}

		e.currBlock.NewBr(e.whileStack[len(e.whileStack)-1].condBlock)
		return nil, lexer.VarType{}
	case *parser.StructStatement:
		typ := &types.StructType{
			TypeName: node.Name,
		}
		fields, names := structFieldMetadata(node)
		for _, t := range fields {
			typ.Fields = append(typ.Fields, e.varTypeToLlvmStructDefn(t, node.Name))
		}
		e.structTypes[node.Name] = typ
		e.structMemberIndexes[node.Name] = names
		e.structMemberTypes[node.Name] = fields
		e.m.NewTypeDef(node.Name, typ)
	case *parser.StructInitializationExpression:
		structType, ok := e.structTypes[node.Name]
		if !ok {
			e.appendError(node.Position(), "couldnt find struct with name %s for initialization", node.Name)
		}
		var fields []constant.Constant
		for _, expr := range node.Values {
			out, _ := e.Emit(expr)
			if cnst, ok := out.(constant.Constant); ok {
				fields = append(fields, cnst)
			} else {
				e.appendError(node.Position(), "non constant field in struct initialization")
			}
		}
		// lol initializing the vartype directly here is far easier (and probably more efficient) than storing it somewhere
		return constant.NewStruct(structType, fields...), lexer.VarType{
			IsStructType: true,
			StructName:   node.Name,
		}
	case *parser.ExternStructStatement:
		typ := &types.StructType{TypeName: node.Name, Opaque: true}
		e.structTypes[node.Name] = typ
		e.m.NewTypeDef(node.Name, typ)
	case *parser.ExternFunctionStatement:
		// TODO: abstract this to an emitdeclare since its used also in the import decls
		var params []*ir.Param
		for _, p := range node.Params {
			params = append(params, ir.NewParam(p.Name.Value, e.varTypeToLlvm(p.Type)))
		}
		fnc := e.m.NewFunc(node.Name, e.varTypeToLlvm(node.ReturnType), params...)
		e.functions[node.Name] = fnc
		e.functionGlReturnTypes[node.Name] = node.ReturnType
	}

	return nil, lexer.VarType{}
}

// this design is a little strange but it becomes very awkward to wire the blocks in this function specifically so i prefer to do it in the callers space and handle the not found return there
func (e *Emitter) emitBlockFindRet(block *parser.BlockStatement) bool {
	for _, s := range block.Statements {
		e.Emit(s)
		if e.currBlock.Term != nil {
			return true
		}
	}
	return false
}

// emitAdress literally only necessary because i need the vptr from ident expr, deref expr is same as e.emit lol
func (e *Emitter) emitAddress(node parser.Node) (value.Value, lexer.VarType) {
	switch node := node.(type) {
	case *parser.IdentifierExpression:
		if param, ok := e.parameters[node.Value]; ok {
			return param, e.parametersGlTypes[node.Value]
		}
		vPtr, ok := e.variables[node.Value]
		if !ok {
			e.appendError(node.Position(), "couldn't find variable with name %s in deref assignment", node.Value)
		}
		vt := e.varGlTypes[node.Value]
		vt.Pointer++
		return vPtr, vt
	case *parser.DereferenceExpression:
		ptr, t := e.Emit(node.Var)
		return ptr, t
	default:
		e.appendError(node.Position(), "invalid node type for emitAddress")
		return nil, lexer.VarType{}
	}
}

func (e *Emitter) emitAsmIntrinsic(pos *util.Position, fnc string, args []parser.Expression) (value.Value, lexer.VarType) {
	switch fnc {
	case "__asm__salloc":
		if len(args) != 2 {
			e.appendError(pos, "invalid amount of arguments for __asm__salloc: %d", len(args))
		}
		size, ok := args[0].(*parser.IntegerLiteral)
		if !ok {
			e.appendError(pos, "first argument of __asm__salloc is not integer: %T", args[0])
		}
		sizeof, ok := args[1].(*parser.SizeofExpression)
		if !ok {
			e.appendError(pos, "second argument of __asm__salloc is not sizeof expr: %T", args[1])
		}
		var arrSize uint64
		if _, ok := glTypeUInts[size.Type.Base]; ok {
			arrSize = size.UValue
		} else {
			arrSize = uint64(size.Value)
		}
		vt := sizeof.Type
		lt := types.NewArray(arrSize, e.varTypeToLlvm(vt))
		ptr := e.currBlock.NewAlloca(lt)
		vt.Pointer++
		return e.currBlock.NewBitCast(ptr, e.varTypeToLlvm(vt)), vt // or gep into elem 0? this seems more suitable..
	default:
		e.appendError(pos, "unknown asm intrinsic function: %s, %v", fnc, args)
		return nil, lexer.VarType{}
	}
}

func (e *Emitter) saveVariableState() *VariableState {
	state := &VariableState{
		variables:  make(map[string]*ir.InstAlloca),
		varTypes:   make(map[string]types.Type),
		varGlTypes: make(map[string]lexer.VarType),
	}

	for k, v := range e.variables {
		state.variables[k] = v
	}

	for k, v := range e.varTypes {
		state.varTypes[k] = v
	}

	for k, v := range e.varGlTypes {
		state.varGlTypes[k] = v
	}

	return state
}

func (e *Emitter) loadVariableState(state *VariableState) {
	e.variables = state.variables
	e.varTypes = state.varTypes
	e.varGlTypes = state.varGlTypes
}

func (e *Emitter) varTypeToLlvmStructDefn(vt lexer.VarType, currStructName string) types.Type {
	// TODO: bit of dupe code here, not sure how to resolve? don't want to integrate the struct stuff into reg vartype resolver as its only for structs
	var baseType types.Type
	if vt.IsStructType {
		find, ok := e.structTypes[vt.StructName]
		if ok {
			baseType = find
		} else {
			if currStructName == vt.StructName {
				baseType = &types.StructType{TypeName: currStructName}
			}
		}
	} else {
		switch vt.Base {
		case lexer.None:
			baseType = nil
		case lexer.Int, lexer.Uint:
			baseType = types.I64
		case lexer.Int32, lexer.Uint32:
			baseType = types.I32
		case lexer.Int8, lexer.Uint8, lexer.Char:
			baseType = types.I8
		case lexer.Int16, lexer.Uint16:
			baseType = types.I16
		case lexer.Void:
			baseType = types.Void
		case lexer.Bool:
			baseType = types.I1
		case lexer.Float:
			baseType = types.Float
		}
	}

	if baseType != nil {
		for _ = range vt.Pointer {
			baseType = &types.PointerType{ElemType: baseType}
		}
	}

	return baseType
}

func (e *Emitter) varTypeToLlvm(vt lexer.VarType) types.Type {
	var baseType types.Type
	if vt.IsStructType {
		baseType = e.structTypes[vt.StructName]
	} else {
		switch vt.Base {
		case lexer.None:
			baseType = nil
		case lexer.Int, lexer.Uint:
			baseType = types.I64
		case lexer.Int32, lexer.Uint32:
			baseType = types.I32
		case lexer.Int8, lexer.Uint8, lexer.Char:
			baseType = types.I8
		case lexer.Int16, lexer.Uint16:
			baseType = types.I16
		case lexer.Void:
			baseType = types.Void
		case lexer.Bool:
			baseType = types.I1
		case lexer.Float:
			baseType = types.Float
		}
	}

	if baseType != nil {
		for _ = range vt.Pointer {
			baseType = &types.PointerType{ElemType: baseType}
		}
	}

	return baseType
}

func (e *Emitter) appendError(pos *util.Position, s string, v ...any) {
	e.Errors = append(e.Errors, util.PositionError{
		Position: pos,
		Msg:      fmt.Sprintf(s, v...),
	})
}

func (e *Emitter) getSizeForVarType(vt lexer.VarType) int64 {
	if vt.Pointer > 0 {
		return 8
	}
	if vt.IsStructType {
		// validated by checker eventually
		stFields, _ := e.structMemberTypes[vt.StructName]
		var sum int64 = 0
		for _, field := range stFields {
			sum += e.getSizeForVarType(field)
		}
		return sum
	}
	switch vt.Base {
	case lexer.Bool, lexer.Int8, lexer.Uint8, lexer.Char:
		return 1
	case lexer.Int16, lexer.Uint16:
		return 2
	case lexer.Int32, lexer.Float, lexer.Uint32:
		return 4
	case lexer.Int, lexer.Uint:
		return 8
	}

	return 0
}

func getSizeForLlvmType(lt types.Type) int64 {
	switch lt := lt.(type) {
	case *types.IntType:
		if lt.BitSize < 8 {
			return 1
		}
		return int64(lt.BitSize / 8)
	case *types.PointerType:
		return 8
	}

	return 0
}

// returns a i64 as thats what the emit integer const function expects, means one less conv
func boolToI1(b bool) int64 {
	if b {
		return 1
	}

	return 0
}
