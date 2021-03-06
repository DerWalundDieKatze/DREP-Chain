package chain

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/drep-project/DREP-Chain/common"

	"github.com/drep-project/DREP-Chain/params"
	"github.com/drep-project/DREP-Chain/types"
)

type ChainBlockValidator struct {
	txValidator ITransactionValidator
	chain       *ChainService
}

func NewChainBlockValidator(chainService *ChainService, txValidator ITransactionValidator) *ChainBlockValidator {
	return &ChainBlockValidator{
		txValidator: txValidator,
		chain:       chainService,
	}
}

func (chainBlockValidator *ChainBlockValidator) VerifyHeader(header, parent *types.BlockHeader) error {
	// Verify chainId  matched
	if header.ChainId != chainBlockValidator.chain.ChainID() {
		return ErrChainId
	}
	// Verify version  matched
	if header.Version != common.Version {
		return ErrVersion
	}
	//Verify header's previousHash is equal parent hash
	if header.PreviousHash != *parent.Hash() {
		return ErrPreHash
	}
	// Verify that the block number is parent's +1
	if header.Height-parent.Height != 1 {
		return ErrInvalidateBlockNumber
	}
	// pre block timestamp before this block time
	if header.Timestamp <= parent.Timestamp {
		return ErrInvalidateTimestamp
	}

	// Verify that the gas limit is <= 2^63-1
	cap := uint64(0x7fffffffffffffff)
	if header.GasLimit.Uint64() > cap {
		return fmt.Errorf("invalid gasLimit: have %v, max %v", header.GasLimit, cap)
	}
	// Verify that the gasUsed is <= gasLimit
	if header.GasUsed.Uint64() > header.GasLimit.Uint64() {
		return fmt.Errorf("invalid gasUsed: have %v, gasLimit %v", header.GasUsed, header.GasLimit)
	}

	//TODO Verify that the gas limit remains within allowed bounds
	nextGasLimit := chainBlockValidator.chain.CalcGasLimit(parent, params.MinGasLimit, params.MaxGasLimit)
	if nextGasLimit.Cmp(&header.GasLimit) != 0 {
		return fmt.Errorf("invalid gas limit: have %v, want %v += %v", header.GasLimit, parent.GasLimit, nextGasLimit)
	}
	return nil
}

func (chainBlockValidator *ChainBlockValidator) VerifyBody(block *types.Block) error {
	// Header validity is known at this point, check the uncles and transactions
	header := block.Header
	if hash := chainBlockValidator.chain.DeriveMerkleRoot(block.Data.TxList); !bytes.Equal(hash, header.TxRoot) {
		return fmt.Errorf("transaction root hash mismatch: have %x, want %x", hash, header.TxRoot)
	}
	return nil
}

func (chainBlockValidator *ChainBlockValidator) ExecuteBlock(context *BlockExecuteContext) error {
	totalGasFee := big.NewInt(0)
	totalGasUsed := big.NewInt(0)
	context.Receipts = make([]*types.Receipt, context.Block.Data.TxCount)
	context.Logs = make([]*types.Log, 0)
	if len(context.Block.Data.TxList) < 0 {
		context.AddGasUsed(totalGasUsed)
		context.AddGasFee(totalGasFee)
		return nil
	}

	for i, t := range context.Block.Data.TxList {
		receipt, gasUsed, gasFee, err := chainBlockValidator.txValidator.ExecuteTransaction(context.Db, t, context.Gp, context.Block.Header)
		if err != nil {
			return err
			//dlog.Debug("execute transaction fail", "txhash", t.Data, "reason", err.Error())
		}
		if gasFee != nil {
			totalGasFee.Add(totalGasFee, gasFee)
			totalGasUsed.Add(totalGasUsed, gasUsed)
		}
		context.Receipts[i] = receipt
		context.Logs = append(context.Logs, receipt.Logs...)
	}
	newReceiptRoot := chainBlockValidator.chain.DeriveReceiptRoot(context.Receipts)
	if newReceiptRoot != context.Block.Header.ReceiptRoot {
		return ErrReceiptRoot
	}
	context.Db.PutReceipts(*context.Block.Header.Hash(), context.Receipts)
	for _, receipt := range context.Receipts {
		context.Db.PutReceipt(receipt.TxHash, receipt)
	}
	context.AddGasUsed(totalGasUsed)
	context.AddGasFee(totalGasFee)
	return nil
}
