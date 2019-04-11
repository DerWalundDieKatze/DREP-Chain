package types

import (
	"github.com/drep-project/drep-chain/app"
	"github.com/drep-project/drep-chain/common"
	"github.com/drep-project/drep-chain/crypto"
	"github.com/drep-project/drep-chain/crypto/secp256k1"
	"github.com/drep-project/drep-chain/crypto/sha3"
	"math/big"
	"time"
	"github.com/drep-project/binary"
)

type Transaction struct {
	Data *TransactionData
	Sig  []byte
}

type TransactionData struct {
	Version   uint64
	Nonce     uint64 //交易序列号
	Type      TxType
	To        crypto.CommonAddress
	ChainId   app.ChainIdType
	Amount    *big.Int
	GasPrice  *big.Int
	GasLimit  *big.Int
	Timestamp uint64
	Data      []byte
	From      crypto.CommonAddress

	// Signature values
	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`
}

func (tx *Transaction) Nonce() uint64 {
	return tx.Data.Nonce
}

func (tx *Transaction) Type() TxType {
	return tx.Data.Type
}

func (tx *Transaction) From() *crypto.CommonAddress {
	return &tx.Data.From
}

type CrossChainTransaction struct {
	ChainId   app.ChainIdType
	StateRoot []byte
	Trans     []*Transaction
}

func (tx *Transaction) GetData() []byte {
	return tx.Data.Data
}

func (tx *Transaction) To() *crypto.CommonAddress {
	return &tx.Data.To
}

func (tx *Transaction) ChainId() app.ChainIdType {
	return tx.Data.ChainId
}

func (tx *Transaction) Amount() *big.Int {
	return tx.Data.Amount
}

func (tx *Transaction) GasLimit() *big.Int {
	return tx.Data.GasLimit
}
func (tx *Transaction) GasPrice() *big.Int {
	return tx.Data.GasPrice
}

//func (tx *Transaction) PubKey() *secp256k1.PublicKey {
//	return tx.Data.PubKey
//}

func (tx *Transaction) TxHash() *crypto.Hash {
	b, _ := binary.Marshal(tx.Data)
	h := sha3.Hash256(b)
	hash := crypto.Hash{}
	hash.SetBytes(h)
	return &hash
}

func (tx *Transaction) TxSig(prvKey *secp256k1.PrivateKey) (*secp256k1.Signature, error) {
	b, err := binary.Marshal(tx.Data)
	if err != nil {
		return nil, err
	}

	return prvKey.Sign(sha3.Hash256(b))
}

func (tx *Transaction) GetGasUsed() *big.Int {
	return new(big.Int).SetInt64(int64(100))
}

func (tx *Transaction) GetGas() *big.Int {
	gasQuantity := tx.GetGasUsed()
	gasUsed := new(big.Int).Mul(gasQuantity, tx.Data.GasPrice)
	return gasUsed
}

func (tx *Transaction) GetSig() []byte {
	return tx.Sig
}

func NewTransaction(from crypto.CommonAddress, to crypto.CommonAddress, amount *big.Int, nonce uint64) *Transaction {
	data := &TransactionData{
		Version:   common.Version,
		Nonce:     nonce,
		Type:      TransferType,
		To:        to,
		Amount:    amount,
		GasPrice:  DefaultGasPrice,
		GasLimit:  TransferGas,
		Timestamp: uint64(time.Now().Unix()),
		From:      from,
	}
	return &Transaction{Data: data}
}

func NewContractTransaction(from crypto.CommonAddress, to crypto.CommonAddress, byteCode []byte, nonce uint64) *Transaction {
	nonce++
	data := &TransactionData{
		Nonce:     nonce,
		Type:      CreateContractType,
		GasPrice:  DefaultGasPrice,
		GasLimit:  CreateContractGas,
		Timestamp: uint64(time.Now().Unix()),
		Data:      make([]byte, len(byteCode)+1),
		From:      from,
	}
	copy(data.Data[1:], byteCode)
	data.Data[0] = 2
	return &Transaction{Data: data}
}

func NewCallContractTransaction(from crypto.CommonAddress, to crypto.CommonAddress, input []byte, amount *big.Int, nonce uint64, readOnly bool) *Transaction {
	nonce++
	data := &TransactionData{
		Nonce:     nonce,
		Type:      CallContractType,
		To:        to,
		Amount:    amount,
		GasPrice:  DefaultGasPrice,
		GasLimit:  CallContractGas,
		Timestamp: uint64(time.Now().Unix()),
		From:      from,
		Data:      make([]byte, len(input)+1),
	}
	copy(data.Data[1:], input)
	if readOnly {
		data.Data[0] = 1
	} else {
		data.Data[0] = 0
	}
	return &Transaction{Data: data}
}
