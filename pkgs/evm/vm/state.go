package vm

import (
	"bytes"
	"math/big"
	"sync"

	"github.com/drep-project/DREP-Chain/crypto"
	"github.com/drep-project/DREP-Chain/database"
	"github.com/drep-project/DREP-Chain/types"
)

var (
	state *State
	once  sync.Once
)

type VMState interface {
	CreateContractAccount(addr crypto.CommonAddress, byteCode []byte) (*types.Account, error)
	SubBalance(addr *crypto.CommonAddress, amount *big.Int) error
	AddBalance(addr *crypto.CommonAddress, amount *big.Int) error
	GetBalance(addr *crypto.CommonAddress) *big.Int
	SetNonce(addr *crypto.CommonAddress, nonce uint64) error
	GetNonce(addr *crypto.CommonAddress) uint64
	Suicide(addr *crypto.CommonAddress) error
	GetByteCode(addr *crypto.CommonAddress) crypto.ByteCode
	GetCodeSize(addr crypto.CommonAddress) int
	GetCodeHash(addr crypto.CommonAddress) crypto.Hash
	SetByteCode(addr *crypto.CommonAddress, byteCode crypto.ByteCode) error
	//GetLogs(txHash crypto.Hash) []*types.Log
	AddLog(contractAddr crypto.CommonAddress, txHash crypto.Hash, data []byte, topics []crypto.Hash, blockNumber uint64) error
	AddRefund(gas uint64)
	SubRefund(gas uint64)
	GetRefund() uint64
	Load(x *big.Int) []byte
	Store(x, y *big.Int)
	Exist(contractAddr crypto.CommonAddress) bool
	Empty(addr *crypto.CommonAddress) bool
	HasSuicided(addr crypto.CommonAddress) bool
}

type State struct {
	db     *database.Database
	refund uint64
	logs   []*types.Log
}

func NewState(database *database.Database) *State {
	return &State{
		db:   database,
		logs: make([]*types.Log, 0),
	}
}
func (s *State) Empty(addr *crypto.CommonAddress) bool {
	so, _ := s.db.GetStorage(addr)
	return so == nil || so.Empty()
}

func (s *State) CreateContractAccount(addr crypto.CommonAddress, byteCode []byte) (*types.Account, error) {
	account, err := types.NewContractAccount(addr)
	if err != nil {
		return nil, err
	}
	account.Storage.ByteCode = byteCode
	return account, s.db.PutStorage(account.Address, account.Storage)
}

func (s *State) SubBalance(addr *crypto.CommonAddress, amount *big.Int) error {
	balance := s.db.GetBalance(addr)
	return s.db.PutBalance(addr, new(big.Int).Sub(balance, amount))
}

func (s *State) AddBalance(addr *crypto.CommonAddress, amount *big.Int) error {
	return s.db.AddBalance(addr, amount)
}

func (s *State) GetBalance(addr *crypto.CommonAddress) *big.Int {
	return s.db.GetBalance(addr)
}

func (s *State) SetNonce(addr *crypto.CommonAddress, nonce uint64) error {
	return s.db.PutNonce(addr, nonce)
}

func (s *State) GetNonce(addr *crypto.CommonAddress) uint64 {
	return s.db.GetNonce(addr)
}

func (s *State) Suicide(addr *crypto.CommonAddress) error {
	return s.db.DeleteStorage(addr)
}

func (s *State) GetByteCode(addr *crypto.CommonAddress) crypto.ByteCode {
	return s.db.GetByteCode(addr)
}

func (s *State) GetCodeSize(addr crypto.CommonAddress) int {
	byteCode := s.GetByteCode(&addr)
	return len(byteCode)

}

func (s *State) GetCodeHash(addr crypto.CommonAddress) crypto.Hash {
	return s.db.GetCodeHash(&addr)
}

func (s *State) SetByteCode(addr *crypto.CommonAddress, byteCode crypto.ByteCode) error {
	return s.db.PutByteCode(addr, byteCode)
}

//TODO test suicided
func (s *State) HasSuicided(addr crypto.CommonAddress) bool {
	storage, err := s.db.GetStorage(&addr)
	if err != nil && storage == nil {
		return true
	}
	return false
}

func (s *State) GetLogs(txHash *crypto.Hash) []*types.Log {
	retLogs := make([]*types.Log, 0)
	for _, log := range s.logs {
		if bytes.Equal(log.TxHash[:], txHash[:]) {
			retLogs = append(retLogs, log)
		}
	}
	return retLogs
}

func (s *State) AddLog(contractAddr crypto.CommonAddress, txHash crypto.Hash, data []byte, topics []crypto.Hash, blockNumber uint64) error {
	log := &types.Log{
		Address: contractAddr,
		TxHash:  txHash,
		Data:    data,
		Topics:  topics,
		Height:  blockNumber,
	}
	s.logs = append(s.logs, log)
	return nil
}

func (s *State) AddRefund(gas uint64) {
	s.refund += gas
}

func (s *State) SubRefund(gas uint64) {
	if gas > s.refund {
		panic("refund below zero")
	}
	s.refund -= gas
}

func (self *State) GetRefund() uint64 {
	return self.refund
}

func (s *State) Load(x *big.Int) []byte {
	return s.db.Load(x)
}

func (s *State) Store(x, y *big.Int) {
	s.db.Store(x, y)
}

func (s *State) Exist(contractAddr crypto.CommonAddress) bool {
	storage, err := s.db.GetStorage(&contractAddr)
	if err != nil || storage == nil {
		return false
	}
	return len(storage.ByteCode) > 0
}
