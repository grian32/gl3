package llvm

import (
	"gl3/hir"

	llvmapi "tinygo.org/x/go-llvm"
)

func (e *Emitter) emitFunction(function *hir.Function) error {
	f := e.functions[function.Id]
	block := llvmapi.AddBasicBlock(f, "body")
	e.builder.SetInsertPointAtEnd(block)
	e.currVaList = llvmapi.Value{}
	if function.Variadic {
		e.currVaList = e.builder.CreateAlloca(e.vaList, "ap")
		e.builder.CreateCall(e.vaStart.GlobalValueType(), e.vaStart, []llvmapi.Value{e.currVaList}, "")
	}

	e.locals = make([]llvmapi.Value, len(function.Locals))
	for i, local := range function.Locals {
		t, err := e.lowerType(local.Type)
		if err != nil {
			return err
		}
		e.locals[i] = e.builder.CreateAlloca(t, local.Name)
	}

	for i := range function.Parameters {
		e.builder.CreateStore(f.Param(i), e.locals[i])
	}

	fallsThrough, err := e.emitBlock(&function.Body)
	if err != nil {
		return err
	}

	if fallsThrough && function.ReturnType.Base == hir.Void && function.ReturnType.Pointer == 0 {
		e.emitVaEnd()
		e.builder.CreateRetVoid()
	}

	return nil
}

func (e *Emitter) emitBlock(block *hir.Block) (bool, error) {
	for _, stmt := range block.Statements {
		fallsThrough, err := e.emitStmt(stmt)
		if err != nil || !fallsThrough {
			return fallsThrough, err
		}
	}

	return true, nil
}

func (e *Emitter) emitVaEnd() {
	if !e.currVaList.IsNil() {
		e.builder.CreateCall(e.vaEnd.GlobalValueType(), e.vaEnd, []llvmapi.Value{e.currVaList}, "")
	}
}

func (e *Emitter) emitStmt(stmt hir.Stmt) (bool, error) {
	switch stmt := stmt.(type) {
	case *hir.LocalDeclaration:
		value, err := e.emitExpr(stmt.Initializer)
		if err != nil {
			return false, err
		}
		e.builder.CreateStore(value, e.locals[stmt.ID])
		return true, nil
	case *hir.Return:
		if stmt.Value == nil {
			e.emitVaEnd()
			e.builder.CreateRetVoid()
			return false, nil
		}
		value, err := e.emitExpr(stmt.Value)
		if err != nil {
			return false, err
		}
		e.emitVaEnd()
		e.builder.CreateRet(value)
		return false, nil
	case *hir.ExpressionStatement:
		_, err := e.emitExpr(stmt.Expr)
		if err != nil {
			return false, err
		}
		return true, nil
	case *hir.If:
		cond, err := e.emitExpr(stmt.Condition)
		if err != nil {
			return false, err
		}
		f := e.builder.GetInsertBlock().Parent()
		thenBlock := llvmapi.AddBasicBlock(f, "if.then")
		endBlock := llvmapi.AddBasicBlock(f, "if.end")
		falseBlock := endBlock
		if stmt.Else != nil {
			falseBlock = llvmapi.AddBasicBlock(f, "if.else")
		}
		e.builder.CreateCondBr(cond, thenBlock, falseBlock)

		e.builder.SetInsertPointAtEnd(thenBlock)
		thenFalls, err := e.emitBlock(&stmt.Then)
		if err != nil {
			return false, err
		}
		if thenFalls {
			e.builder.CreateBr(endBlock)
		}

		if stmt.Else != nil {
			e.builder.SetInsertPointAtEnd(falseBlock)
			elseFalls, err := e.emitBlock(stmt.Else)
			if err != nil {
				return false, err
			}
			if elseFalls {
				e.builder.CreateBr(endBlock)
			}
			if !thenFalls && !elseFalls {
				endBlock.EraseFromParent()
				return false, nil
			}
		}

		e.builder.SetInsertPointAtEnd(endBlock)
		return true, nil
	case *hir.While:
		f := e.builder.GetInsertBlock().Parent()
		cond := llvmapi.AddBasicBlock(f, "while.cond")
		body := llvmapi.AddBasicBlock(f, "while.body")
		end := llvmapi.AddBasicBlock(f, "while.end")

		e.builder.CreateBr(cond)

		e.builder.SetInsertPointAtEnd(cond)
		cExpr, err := e.emitExpr(stmt.Condition)
		if err != nil {
			return false, err
		}
		e.builder.CreateCondBr(cExpr, body, end)

		e.loops = append(e.loops, loopBlocks{
			condition: cond,
			exit:      end,
		})
		e.builder.SetInsertPointAtEnd(body)
		falls, err := e.emitBlock(&stmt.Body)
		if err != nil {
			return false, err
		}
		if falls {
			e.builder.CreateBr(cond)
		}
		e.loops = e.loops[:len(e.loops)-1]
		e.builder.SetInsertPointAtEnd(end)

		return true, nil
	case *hir.Break:
		e.builder.CreateBr(e.loops[len(e.loops)-1].exit)
		return false, nil
	case *hir.Continue:
		e.builder.CreateBr(e.loops[len(e.loops)-1].condition)
		return false, nil
	}
	panic("emitter: emitStmt not implemented")
}
