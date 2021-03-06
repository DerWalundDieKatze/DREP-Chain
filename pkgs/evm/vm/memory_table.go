package vm

import (
	"github.com/drep-project/DREP-Chain/common"
	"math/big"
)

func memorySha3(stack *Stack) *big.Int {
	return common.CalcMemSize(stack.Back(0), stack.Back(1))
}

func memoryCallDataCopy(stack *Stack) *big.Int {
	return common.CalcMemSize(stack.Back(0), stack.Back(2))
}

func memoryReturnDataCopy(stack *Stack) *big.Int {
	return common.CalcMemSize(stack.Back(0), stack.Back(2))
}

func memoryCodeCopy(stack *Stack) *big.Int {
	return common.CalcMemSize(stack.Back(0), stack.Back(2))
}

func memoryExtCodeCopy(stack *Stack) *big.Int {
	return common.CalcMemSize(stack.Back(1), stack.Back(3))
}

func memoryMLoad(stack *Stack) *big.Int {
	return common.CalcMemSize(stack.Back(0), big.NewInt(32))
}

func memoryMStore8(stack *Stack) *big.Int {
	return common.CalcMemSize(stack.Back(0), big.NewInt(1))
}

func memoryMStore(stack *Stack) *big.Int {
	return common.CalcMemSize(stack.Back(0), big.NewInt(32))
}

func memoryCreate(stack *Stack) *big.Int {
	return common.CalcMemSize(stack.Back(1), stack.Back(2))
}

func memoryCreate2(stack *Stack) *big.Int {
	return common.CalcMemSize(stack.Back(1), stack.Back(2))
}

func memoryCall(stack *Stack) *big.Int {
	x := common.CalcMemSize(stack.Back(5), stack.Back(6))
	y := common.CalcMemSize(stack.Back(3), stack.Back(4))

	return common.BigMax(x, y)
}

func memoryDelegateCall(stack *Stack) *big.Int {
	x := common.CalcMemSize(stack.Back(4), stack.Back(5))
	y := common.CalcMemSize(stack.Back(2), stack.Back(3))

	return common.BigMax(x, y)
}

func memoryStaticCall(stack *Stack) *big.Int {
	x := common.CalcMemSize(stack.Back(4), stack.Back(5))
	y := common.CalcMemSize(stack.Back(2), stack.Back(3))

	return common.BigMax(x, y)
}

func memoryReturn(stack *Stack) *big.Int {
	return common.CalcMemSize(stack.Back(0), stack.Back(1))
}

func memoryRevert(stack *Stack) *big.Int {
	return common.CalcMemSize(stack.Back(0), stack.Back(1))
}

func memoryLog(stack *Stack) *big.Int {
	mSize, mStart := stack.Back(1), stack.Back(0)
	return common.CalcMemSize(mStart, mSize)
}
