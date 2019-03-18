package service

import (
	"encoding/hex"
	"errors"
	"math/big"

	chainService "github.com/drep-project/drep-chain/chain/service"
	chainTypes "github.com/drep-project/drep-chain/chain/types"
	"github.com/drep-project/drep-chain/crypto"
	"github.com/drep-project/drep-chain/crypto/secp256k1"
	"github.com/drep-project/drep-chain/crypto/sha3"
	"github.com/drep-project/drep-chain/database"
)

type AccountApi struct {
	Wallet          *Wallet
	accountService  *AccountService
	chainService    *chainService.ChainService
	databaseService *database.DatabaseService
}

func (accountapi *AccountApi) AddressList() ([]*secp256k1.PublicKey, error) {
	if !accountapi.Wallet.IsOpen() {
		return nil, errors.New("wallet is not open")
	}
	return accountapi.Wallet.ListKeys()
}

func (accountapi *AccountApi) CreateWallet(password string) error {
	wallet, err := CreateWallet(accountapi.accountService.Config, accountapi.chainService.Config.ChainId,password)
	if err != nil {
		return err
	}
	accountapi.Wallet = wallet
	return nil
}

// Lock lock the wallet to protect private key
func (accountapi *AccountApi) LockWallet() error {
	if !accountapi.Wallet.IsOpen() {
		return errors.New("wallet is not open")
	}
	if !accountapi.Wallet.IsLock() {
		return accountapi.Wallet.Lock()
	}
	return errors.New("wallet is already locked")
}

// UnLock unlock the wallet
func (accountapi *AccountApi) UnLockWallet(password string) error {
	if !accountapi.Wallet.IsOpen() {
		return errors.New("wallet is not open")
	}
	if accountapi.Wallet.IsLock() {
		return accountapi.Wallet.UnLock(password)
	}
	return errors.New("wallet is already unlock")
}

func (accountapi *AccountApi) OpenWallet(password string) error {
	return accountapi.Wallet.Open(password)
}

func (accountapi *AccountApi) CloseWallet() {
	accountapi.Wallet.Close()
}

func (accountapi *AccountApi) SendTransaction(from, to string, amount *big.Int) (string, error) {
	nonce := accountapi.chainService.GetTransactionCount(from)
	t := chainTypes.NewTransaction(from, to, amount, nonce)
	err := accountapi.chainService.SendTransaction(t)
	if err != nil{
		return "",err
	}
	txHash, err := t.TxHash()
	if err != nil{
		return "",err
	}

	hex := hex.EncodeToString(txHash)
	//bytes, _ := json.Marshal(t)
	//println(string(bytes))
	//println("0x" + string(hex))
	return "0x" + string(hex), nil
}

func (accountapi *AccountApi) Call(from, to string, input []byte, amount *big.Int, readOnly bool) (string, error) {
	nonce := accountapi.chainService.GetTransactionCount(from)
	t := chainTypes.NewCallContractTransaction(from, to, input, amount, nonce, readOnly)
	accountapi.chainService.SendTransaction(t)
	return t.TxId()
}

func (accountapi *AccountApi) CreateCode(from, to string, byteCode []byte) (string, error) {
	nonce := accountapi.chainService.GetTransactionCount(from)
	t := chainTypes.NewCreateContractTransaction(from, to, byteCode, nonce)
	accountapi.chainService.SendTransaction(t)
	return t.TxId()
}

// DumpPrikey dumpPrivate
func (accountapi *AccountApi) DumpPrivkey(address *secp256k1.PublicKey) (*secp256k1.PrivateKey, error) {
	if !accountapi.Wallet.IsOpen() {
		return nil, errors.New("wallet is not open")
	}
	if accountapi.Wallet.IsLock() {
		return nil, errors.New("wallet has locked")
	}

	key, err := accountapi.Wallet.DumpPrivateKey(address)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func (accountapi *AccountApi) Sign(key *secp256k1.PublicKey, msg string) ([]byte, error) {
	prv, _ := accountapi.DumpPrivkey(key)
	bytes := sha3.Hash256([]byte(msg))
	return crypto.Sign(bytes, prv)
}

func (accountapi *AccountApi) GasPrice() *big.Int {
	return chainTypes.DefaultGasPrice
}

func (accountapi *AccountApi) GetCode(accountName string) []byte {
	return accountapi.databaseService.GetByteCode(accountName, false)
}