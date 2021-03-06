package chain

import (
	"math/big"

	"github.com/drep-project/DREP-Chain/common"
	"github.com/drep-project/DREP-Chain/crypto"
	"github.com/drep-project/DREP-Chain/database"
	"github.com/drep-project/DREP-Chain/database/drepdb/memorydb"
	"github.com/drep-project/DREP-Chain/params"
	types "github.com/drep-project/DREP-Chain/types"
)

func (chainService *ChainService) GetGenisiBlock(biosAddress crypto.CommonAddress) *types.Block {
	var root []byte
	db, _ := database.DatabaseFromStore(memorydb.New())
	for addr, balance := range params.Preminer {
		//add preminer addr and balance
		storage := types.NewStorage()
		storage.Balance = *balance
		db.PutStorage(&addr, storage)
	}
	root = db.GetStateRoot()

	merkleRoot := chainService.DeriveMerkleRoot(nil)
	return &types.Block{
		Header: &types.BlockHeader{
			Version:      common.Version,
			PreviousHash: crypto.Hash{},
			GasLimit:     *new(big.Int).SetUint64(params.GenesisGasLimit),
			GasUsed:      *new(big.Int),
			Timestamp:    1545282765,
			StateRoot:    root,
			TxRoot:       merkleRoot,
			Height:       0,
		},
		Data: &types.BlockData{
			TxCount: 0,
			TxList:  []*types.Transaction{},
		},
	}
}

func (chainService *ChainService) ProcessGenesisBlock(biosAddr crypto.CommonAddress) (*types.Block, error) {
	var err error
	var root []byte

	for addr, balance := range params.Preminer {
		//add preminer addr and balance
		storage := types.NewStorage()
		storage.Balance = *balance
		chainService.DatabaseService.PutStorage(&addr, storage)
	}

	root = chainService.DatabaseService.GetStateRoot()
	if err != nil {
		return nil, err
	}

	chainService.DatabaseService.Commit()
	triedb := chainService.DatabaseService.GetTriedDB()
	triedb.Commit(crypto.Bytes2Hash(root), true)

	merkleRoot := chainService.DeriveMerkleRoot(nil)
	return &types.Block{
		Header: &types.BlockHeader{
			Version:      common.Version,
			PreviousHash: crypto.Hash{},
			GasLimit:     *new(big.Int).SetUint64(params.GenesisGasLimit),
			GasUsed:      *new(big.Int),
			Timestamp:    1545282765,
			StateRoot:    root,
			TxRoot:       merkleRoot,
			Height:       0,
		},
		Data: &types.BlockData{
			TxCount: 0,
			TxList:  []*types.Transaction{},
		},
	}, nil
}
