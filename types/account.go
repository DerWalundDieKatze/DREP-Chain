package types

import (
	"bytes"
	"github.com/drep-project/drep-chain/common"
	"github.com/drep-project/drep-chain/crypto"
	"github.com/drep-project/drep-chain/crypto/secp256k1"
	"github.com/drep-project/drep-chain/crypto/sha3"
	"math/big"
)

var (
	DrepMark      = []byte("Drep Coin Seed")
	KeyBitSize    = 256 >> 3
	emptyCodeHash = sha3.Keccak256(nil)
)

type Node struct {
	Address    *crypto.CommonAddress
	PrivateKey *secp256k1.PrivateKey
	ChainId    ChainIdType
	ChainCode  []byte
}

func NewNode(parent *Node, chainId ChainIdType) *Node {
	var (
		prvKey    *secp256k1.PrivateKey
		chainCode []byte
	)

	IsRoot := parent == nil
	if IsRoot {
		uni, err := common.GenUnique()
		if err != nil {
			return nil
		}
		h := common.HmAC(uni, DrepMark)
		prvKey, _ = crypto.ToPrivateKey(h[:KeyBitSize])
		chainCode = h[KeyBitSize:]
	} else {
		pid := new(big.Int).SetBytes(parent.ChainCode)
		cid := new(big.Int).SetBytes(chainId.Bytes())
		chainCode := new(big.Int).Xor(pid, cid).Bytes()

		h := common.HmAC(chainCode, parent.PrivateKey.Serialize())
		prvKey, _ = crypto.ToPrivateKey(h[:KeyBitSize])
		chainCode = h[KeyBitSize:]
	}
	address := crypto.PubkeyToAddress(prvKey.PubKey())
	return &Node{
		Address:    &address,
		PrivateKey: prvKey,
		ChainId:    chainId,
		ChainCode:  chainCode,
	}
}

type Storage struct {
	Balance    big.Int
	Reputation big.Int

	Nonce uint64
	//contract
	ByteCode crypto.ByteCode
	CodeHash crypto.Hash

	Alias      string
	BalanceMap map[string]big.Int

	ReceivedVoteCredit map[crypto.CommonAddress]big.Int //Trust given by oneself and others
	SentVoteCredit     map[crypto.CommonAddress]big.Int //Vote for Trust to Address
	CancelVoteCredit   map[*big.Int]big.Int             //撤销给与别人的信任数据存放于此；等待一段时间或者高度后，value对应的balance加入到Balance中。key是撤销时交易所在的高度

	//LockBalance        map[crypto.CommonAddress]big.Int //智能合约中即可
}

func NewStorage() *Storage {
	storage := &Storage{}
	storage.Nonce = 0
	return storage
}

type Account struct {
	Address *crypto.CommonAddress
	Node    *Node
	Storage *Storage
}

func (account *Account) Sign(hash []byte) ([]byte, error) {
	return crypto.Sign(hash, account.Node.PrivateKey)
}

func (s *Storage) Empty() bool {
	return s.Nonce == 0 && s.Balance.Sign() == 0 && bytes.Equal(s.CodeHash[:], emptyCodeHash)
}

func NewNormalAccount(parent *Node, chainId ChainIdType) (*Account, error) {
	/*IsRoot := chainId == RootChain
	if !IsRoot && parent == nil {
		return nil, errors.New("missing parent account")
	}*/
	node := NewNode(parent, chainId)
	address := node.Address
	storage := NewStorage()
	account := &Account{
		Address: address,
		Node:    node,
		Storage: storage,
	}
	return account, nil
}

func NewContractAccount(address crypto.CommonAddress) (*Account, error) {
	storage := NewStorage()
	account := &Account{
		Address: &address,
		Node:    &Node{},
		Storage: storage,
	}
	return account, nil
}
