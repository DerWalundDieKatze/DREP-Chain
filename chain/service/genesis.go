package service

import (
	chainTypes "github.com/drep-project/drep-chain/chain/types"
	"github.com/drep-project/drep-chain/common"
	"github.com/drep-project/drep-chain/chain/params"
	"github.com/drep-project/drep-chain/crypto"
	"github.com/drep-project/drep-chain/crypto/secp256k1"
	"github.com/drep-project/drep-chain/database"
	"math/big"
)

func (chainService *ChainService) GetGenisiBlock(genesisPubkey string) *chainTypes.Block {
	var root []byte
	//NOTICE pre mine
	db, err := database.DatabaseFromStore(database.NewMemoryStore())
	for _, producer := range chainService.Config.Producers {
		//add account
		storage := chainTypes.NewStorage()
		storage.Balance = *big.NewInt(0).Mul(big.NewInt(1000000000000000000), big.NewInt(1000000000))
		addr := crypto.PubKey2Address(producer.Pubkey)
		db.PutStorage(&addr, storage)
	}
	root = db.GetStateRoot()

	merkleRoot := chainService.deriveMerkleRoot(nil)
	b := common.MustDecode(genesisPubkey)
	pubkey, err := secp256k1.ParsePubKey(b)
	if err != nil {
		return nil
	}
	return &chainTypes.Block{
		Header: &chainTypes.BlockHeader{
			Version:      common.Version,
			PreviousHash: crypto.Hash{},
			GasLimit:     *new (big.Int).SetUint64(params.GenesisGasLimit),
			GasUsed:      *new(big.Int),
			Timestamp:    1545282765,
			StateRoot:    root,
			TxRoot:       merkleRoot,
			Height:       0,
			LeaderPubKey: *pubkey,
		},
		Data: &chainTypes.BlockData{
			TxCount: 0,
			TxList:  []*chainTypes.Transaction{},
		},
	}
}

func (chainService *ChainService) ProcessGenesisBlock(genesisPubkey string) (*chainTypes.Block, error) {
	var err error
	var root []byte
	//NOTICE pre mine
	for _, producer := range chainService.Config.Producers {
		//add account
		storage := chainTypes.NewStorage()
		storage.Balance = *big.NewInt(0).Mul(big.NewInt(1000000000000000000), big.NewInt(1000000000))
		addr := crypto.PubKey2Address(producer.Pubkey)
		chainService.DatabaseService.PutStorage(&addr, storage)
	}
	root = chainService.DatabaseService.GetStateRoot()

	if err != nil {
		return nil, err
	}

	merkleRoot := chainService.deriveMerkleRoot(nil)
	b := common.MustDecode(genesisPubkey)
	pubkey, err := secp256k1.ParsePubKey(b)
	if err != nil {
		return nil, err
	}
	chainService.DatabaseService.RecordBlockJournal(0)
	return &chainTypes.Block{
		Header: &chainTypes.BlockHeader{
			Version:      common.Version,
			PreviousHash: crypto.Hash{},
			GasLimit:     *new (big.Int).SetUint64(params.GenesisGasLimit),
			GasUsed:      *new(big.Int),
			Timestamp:    1545282765,
			StateRoot:    root,
			TxRoot:       merkleRoot,
			Height:       0,
			LeaderPubKey: *pubkey,
		},
		Data: &chainTypes.BlockData{
			TxCount: 0,
			TxList:  []*chainTypes.Transaction{},
		},
	}, nil
}