package service

import (
	"errors"
	"github.com/drep-project/binary"
	"github.com/drep-project/dlog"
	"github.com/drep-project/drep-chain/app"
	"github.com/drep-project/drep-chain/chain/params"
	chainTypes "github.com/drep-project/drep-chain/chain/types"
	"github.com/drep-project/drep-chain/crypto"
	"github.com/drep-project/drep-chain/crypto/secp256k1"
	"github.com/drep-project/drep-chain/pkgs/evm/vm"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	allowedFutureBlockTime    = 15 * time.Second
)

var (
	childTrans []*chainTypes.Transaction
	errBalance = errors.New("not enough balance")
)

func (chainService *ChainService) VerifyTransaction(tx *chainTypes.Transaction) error {
	return chainService.verifyTransaction(tx)
}

func (chainService *ChainService) verifyTransaction(tx *chainTypes.Transaction) error {
	var result error
	_ = chainService.DatabaseService.Transaction(func() error {
		from := tx.From()
		nounce := tx.Nonce()

		_, err := chainService.verify(tx)
		if err != nil {
			result = err
		}

		err = chainService.checkNonce(from, nounce)
		if err != nil {
			result =  err
		}

		// Check the transaction doesn't exceed the current
		// block limit gas.
		block, _ := chainService.GetBlockByHash(chainService.BestChain.Tip().Hash)
		if block.Header.GasLimit.Uint64() < tx.Gas() {
			result = errors.New("gas limit in tx has exceed block limit")
		}

		// Transactions can't be negative. This may never happen
		// using RLP decoded transactions but may occur if you create
		// a transaction using the RPC for example.
		if tx.Amount().Sign() < 0 {
			result = errors.New("negative amount in tx")
		}

		// Transactor should have enough funds to cover the costs
		// cost == V + GP * GL
		originBalance := chainService.DatabaseService.GetBalance(from, true)
		if originBalance.Cmp(tx.Cost()) < 0 {
			result = errors.New("not enough balance")
		}

		// Should supply enough intrinsic gas
		gas, err := IntrinsicGas(tx.AsPersistentMessage(), tx.To() == nil|| tx.To().IsEmpty() )
		if err != nil {
			result = err
		}
		if tx.Gas() < gas {
			result = errors.New("not enough balance")
		}
		return errors.New("just not commit")
	})
	return result
}

//TODO 交易验证存在的问题， 合约是否需要执行
func (chainService *ChainService) executeTransaction(tx *chainTypes.Transaction, gp *GasPool, header *chainTypes.BlockHeader) (*big.Int, *big.Int, error) {
	//gp       = new(GasPool).AddGas(block.GasLimit())
	newState := vm.NewState(chainService.DatabaseService)
	_, err := chainService.verify(tx)
	if err != nil {
		return nil, nil, err
	}

	//TODO need test
	gasUsed := new(uint64)
	_, _, err = chainService.stateProcessor.ApplyTransaction(newState, chainService, gp, header,tx, gasUsed)
	if err != nil {
		dlog.Error("executeTransaction transaction error", "reason", err)
		return nil, nil, err
	}
	gasFee := new (big.Int).Mul(new(big.Int).SetUint64(*gasUsed), tx.GasPrice())
	return new(big.Int).SetUint64(*gasUsed), gasFee, nil
}

func (chainService *ChainService) verify(tx *chainTypes.Transaction) (bool, error){
	if tx.Sig != nil {
		pk, _, err := secp256k1.RecoverCompact(tx.Sig, tx.TxHash().Bytes())
		if err != nil {
			return false, err
		}
		sig := secp256k1.RecoverSig(tx.Sig)
		isValid := sig.Verify(tx.TxHash().Bytes(), pk)
		if err != nil {
			return false, err
		}
		if !isValid {
			return false, errors.New("signature not validate")
		}
		return true, nil
	}else{
		return false, errors.New("must assign a signature for transaction")
	}
}

//func (chainService *ChainService) executeSetAliasTransaction(t *chainTypes.Transaction, fromAccount *crypto.CommonAddress, alias string, gasPrice, gasLimit *big.Int, chainId app.ChainIdType) (*big.Int, *big.Int, error) {
//
//	//2 设置
//	err := chainService.DatabaseService.AliasSet(fromAccount,alias)
//	return chainTypes.GasTable[chainTypes.SetAliasType], gasPrice, err
//}

func (chainService *ChainService) checkNonce(fromAccount *crypto.CommonAddress, nounce uint64) error {
	nonce := chainService.DatabaseService.GetNonce(fromAccount, true)
	if nonce > nounce {
		return errors.New("error nounce")
	}
	return nil
}

func (chainService *ChainService) checkBalance(gaslimit, gasPrice, balance, gasFloor, gasCap *big.Int) error {
	if gasFloor != nil {
		amountFloor := new(big.Int).Mul(gasFloor, gasPrice)
		if gaslimit.Cmp(gasFloor) < 0 || amountFloor.Cmp(balance) > 0 {
			return errors.New("not enough gas")
		}
	}
	if gasCap != nil {
		amountCap := new(big.Int).Mul(gasCap, gasPrice)
		if amountCap.Cmp(balance) > 0 {
			return errors.New("too much gaslimit")
		}
	}
	return nil
}

func (chainService *ChainService) deduct(chainId app.ChainIdType, balance, gasFee *big.Int) (leftBalance, actualFee *big.Int) {
	leftBalance = new(big.Int).Sub(balance, gasFee)
	actualFee = new(big.Int).Set(gasFee)
	if leftBalance.Sign() < 0 {
		actualFee = new(big.Int).Set(balance)
		leftBalance = new(big.Int)
	}
	return leftBalance, actualFee
}

func (chainService *ChainService) preSync(block *chainTypes.Block) {
	if !chainService.isRelay && chainService.chainId != chainService.RootChain() {
		return
	}
	if childTrans == nil {
		childTrans = make([]*chainTypes.Transaction, 0)
	}
	childTrans = append(childTrans, block.Data.TxList...)
}

func (chainService *ChainService) doSync(height uint64) {
	if !chainService.isRelay || chainService.chainId == chainService.RootChain() || height%2 != 0 || height == 0 {
		return
	}
	cct := &chainTypes.CrossChainTransaction{
		ChainId:   chainService.chainId,
		StateRoot: chainService.DatabaseService.GetStateRoot(),
		Trans:     childTrans,
	}
	data, err := binary.Marshal(cct)
	if err != nil {
		return
	}
	values := url.Values{}
	values.Add("data", string(data))
	body := values.Encode()
	urlStr := "http://localhost:" + strconv.Itoa(chainService.Config.RemotePort) + "/SyncChildChain?" + body
	http.Get(urlStr)
	childTrans = nil
}

// IntrinsicGas computes the 'intrinsic gas' for a message with the given data.
func IntrinsicGas(data []byte, contractCreation bool) (uint64, error) {
	// Set the starting gas for the raw transaction
	var gas uint64
	if contractCreation {
		gas = params.TxGasContractCreation
	} else {
		gas = params.TxGas
	}
	// Bump the required gas by the amount of transactional data
	if len(data) > 0 {
		// Zero and non-zero bytes are priced differently
		var nz uint64
		for _, byt := range data {
			if byt != 0 {
				nz++
			}
		}
		// Make sure we don't exceed uint64 for all data combinations
		if (math.MaxUint64-gas)/params.TxDataNonZeroGas < nz {
			return 0, vm.ErrOutOfGas
		}
		gas += nz * params.TxDataNonZeroGas

		z := uint64(len(data)) - nz
		if (math.MaxUint64-gas)/params.TxDataZeroGas < z {
			return 0, vm.ErrOutOfGas
		}
		gas += z * params.TxDataZeroGas
	}
	return gas, nil
}












//func (chainService *ChainService) executeCrossChainTransaction(t *chainTypes.Transaction) (gasUsed *big.Int, gasFee *big.Int) {
//    var (
//        can bool
//        addr crypto.CommonAddress
//        balance, gasPrice *big.Int
//    )
//
//    gasUsed, gasFee = new(big.Int), new(big.Int)
//    can, addr,  _, _, gasPrice = chainService.canExecute(t, nil, CrossChainGas)
//    if !can {
//        return new(big.Int), new(big.Int)
//    }
//
//    cct := &chainTypes.CrossChainTransaction{}
//    err := json.Unmarshal(t.Data.Data, cct)
//    if err != nil {
//        fmt.Println("err: ", err)
//        return new(big.Int), new(big.Int)
//    }
//
//    gasSum := new(big.Int)
//    for _, tx := range cct.Trans {
//       if tx.Data.Type == CrossChainType {
//           continue
//       }
//       g, _ := chainService.executeTransaction(tx)
//       gasSum = new(big.Int).Add(gasSum, g)
//    }
//
//    if !bytes.Equal(chainService.databaseService.GetStateRoot(), cct.StateRoot) {
//       //subDt.Discard()
//    } else {
//        amountSum := new(big.Int).Mul(gasSum, gasPrice)
//        balance = chainService.databaseService.GetBalance(addr, t.Data.ChainId, true)
//        if balance.Cmp(amountSum) >= 0 {
//            gasUsed = new(big.Int).Set(gasSum)
//            gasFee = new(big.Int).Set(amountSum)
//            _, gasFee = chainService.deduct(addr, t.Data.ChainId, balance, gasFee)
//            //subDt.Commit()
//        } else {
//            //subDt.Discard()
//        }
//    }
//    return
//}

//func preExecuteCrossChainTransaction(dt database.Transactional, t *chainTypes.Transaction) (gasUsed, gasFee *big.Int) {
//    var (
//        can bool
//        addr crypto.CommonAddress
//        balance, gasPrice *big.Int
//    )
//
//    gasUsed, gasFee = new(big.Int), new(big.Int)
//    subDt := dt.BeginTransaction()
//    can, addr,  _, _, gasPrice = canExecute(subDt, t, nil, CrossChainGas)
//    if !can {
//        return new(big.Int), new(big.Int)
//    }
//
//    cct := &chainTypes.CrossChainTransaction{}
//    err := json.Unmarshal(t.Data.Data, &cct)
//    if err != nil {
//        return new(big.Int), new(big.Int)
//    }
//
//    gasSum := new(big.Int)
//    for _, tx := range cct.Trans {
//        if tx.Data.Type == CrossChainType {
//            continue
//        }
//        g, _ := executeTransaction(subDt, tx)
//        gasSum = new(big.Int).Add(gasSum, g)
//    }
//
//    cct.StateRoot = subDt.GetChainStateRoot(database.ChildCHAIN)
//    t.Data.Data, _ = json.Marshal(cct)
//
//    amountSum := new(big.Int).Mul(gasSum, gasPrice)
//    balance = database.GetBalance(addr, t.Data.ChainId)
//    if balance.Cmp(amountSum) >= 0 {
//        gasUsed = new(big.Int).Set(gasSum)
//        gasFee = new(big.Int).Set(amountSum)
//        _, gasFee = deduct(subDt, addr, t.Data.ChainId, balance, gasFee)
//        subDt.Commit()
//    } else {
//        subDt.Discard()
//    }
//
//    return
//}
