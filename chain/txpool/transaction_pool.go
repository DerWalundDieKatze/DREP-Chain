package txpool

import (
	"container/heap"
	"errors"
	"fmt"
	"github.com/drep-project/dlog"
	chainTypes "github.com/drep-project/drep-chain/chain/types"
	"github.com/drep-project/drep-chain/common/event"
	"github.com/drep-project/drep-chain/database"
	"math/big"
	"sync"
)

const maxSize = 100000

//1 池子里的交易按照nonce是否连续，分为乱序的和已经排序的在两个不同的队列中
//2 已经排序好的可以被打包入块
//3 池子里面的交易根据块中的各个地址的交易对应的Nonce进行删除

type TransactionPool struct {
	databaseApi *database.DatabaseService

	//trans       *list.LinkedList
	queue   map[string]*txList
	pending map[string]*txList
	//accountTran map[crypto.CommonAddress]*list.SortedLinkedList
	allTxs  map[string]bool
	mu      sync.Mutex
	nonceCp func(a interface{}, b interface{}) int
	tranCp  func(a interface{}, b interface{}) bool

	//当前有序的最大的nonce大小,此值应该被存储到DB中（后续考虑txpool的DB存储，一起考虑）
	pendingNonce     map[string]int64
	eventNewBlockSub event.Subscription
	newBlockChan     chan []string
	quit             chan struct{}
}

func NewTransactionPool(databaseApi *database.DatabaseService) *TransactionPool {
	pool := &TransactionPool{databaseApi: databaseApi}
	pool.nonceCp = func(a interface{}, b interface{}) int {
		ta, oka := a.(*chainTypes.Transaction)
		tb, okb := b.(*chainTypes.Transaction)
		if oka && okb {
			nonceA := ta.Nonce()
			nonceB := tb.Nonce()
			if nonceA < nonceB {
				return -1
			} else if nonceA > nonceB {
				return 1
			} else {
				return 0
			}
		} else {
			return 0
		}
	}
	pool.tranCp = func(a interface{}, b interface{}) bool {
		ta, oka := a.(*chainTypes.Transaction)
		tb, okb := b.(*chainTypes.Transaction)
		sa, ea := ta.TxId()
		sb, eb := tb.TxId()
		return oka && okb && ea == nil && eb == nil && sa == sb
	}

	pool.allTxs = make(map[string]bool)
	pool.queue = make(map[string]*txList)
	pool.pending = make(map[string]*txList)
	pool.newBlockChan = make(chan []string)
	pool.pendingNonce = make(map[string]int64)

	return pool
}

func (pool *TransactionPool) Contains(id string) bool {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	value, exists := pool.allTxs[id]
	if exists && !value {
		delete(pool.allTxs, id)
	}
	return exists || value
}

func (pool *TransactionPool) checkAndGetAddr(tx *chainTypes.Transaction) (error, string) {
	accountName := tx.From()
	// TODO Check sig
	if pool.getTransactionCount(accountName) > tx.Nonce() {
		fmt.Println("checkAndGetAddr:", pool.getTransactionCount(accountName), tx.Nonce())
		return fmt.Errorf("nonce err ,dbNonce:%d > txNonce:%d", pool.getTransactionCount(accountName), tx.Nonce()), ""
	}

	amount := new(big.Int).SetBytes(tx.Amount().Bytes())
	gasLimit := new(big.Int).SetBytes(tx.GasLimit().Bytes())
	gasPrice := new(big.Int).SetBytes(tx.GasPrice().Bytes())
	total := big.NewInt(0)
	total.Mul(gasLimit, gasPrice)
	total.Add(total, amount)

	addrBalance := pool.databaseApi.GetBalance(accountName, false)
	if addrBalance.Cmp(total) < 0 {
		dlog.Debug("Not Enough Balance", "CurrentBalance", addrBalance.Int64(), "NeedBalance", total)
		return fmt.Errorf("no enough balance"), ""
	}

	return nil, accountName
}

//func AddTransaction(id string, transaction *common.transaction) {
func (pool *TransactionPool) AddTransaction(tx *chainTypes.Transaction) error {
	err, accountName := pool.checkAndGetAddr(tx)
	if err != nil {
		return err
	}
	id, err := tx.TxId()
	if err != nil {
		return err
	}
	pool.mu.Lock()
	defer pool.mu.Unlock()
	if len(pool.allTxs) >= maxSize {
		msg := fmt.Sprintf("transaction pool full.txid:%s fail to add.pool tx count:%d, maxSize:%d",id, len(pool.allTxs),maxSize)
		dlog.Error(msg)
		return errors.New(msg)
	}

	if _, exists := pool.allTxs[id]; exists {
		msg := "transaction %s exists" + id
		dlog.Error("transaction %s exists", id)
		return errors.New(msg)
	} else {
		pool.allTxs[id] = true

		if list, ok := pool.queue[tx.From()]; ok {
			list.Add(tx)
		} else {
			pool.queue[tx.From()] = newTxList(true)
			pool.queue[tx.From()].Add(tx)
		}

		pool.syncToPending(accountName)
	}
	return nil
}

func (pool *TransactionPool) syncToPending(accountName string) {
	//从queue找nonce连续的交易放入到pending中
	list := pool.queue[accountName].Ready(pool.getTransactionCount(accountName))

	if _, ok := pool.pending[accountName]; !ok {
		pool.pending[accountName] = newTxList(true)
	}

	var nonce int64
	listPending := pool.pending[accountName]
	if len(list) > 0 {
		for _, tx := range list {
			listPending.Add(tx)
			nonce = tx.Nonce() + 1
		}

		pool.pendingNonce[accountName] = nonce
	}
}

func (pool *TransactionPool) removeTransaction(tran *chainTypes.Transaction) (bool, bool) {
	//id, err := tran.TxId()
	//if err != nil {
	//	return false, false
	//}
	//pool.tranLock.Lock()
	//defer pool.tranLock.Unlock()
	//r1 := pool.trans.Remove(tran, pool.tranCp)
	//delete(pool.allTxs, id)
	//addr := crypto.PubKey2Address(tran.Data.PubKey)
	//ts := pool.accountTran[addr]
	//r2 := ts.Remove(tran, pool.tranCp)
	//return r1, r2
	return true, true
}

//打包过程获取交易，进行打包处理
func (pool *TransactionPool) GetPending(GasLimit *big.Int) []*chainTypes.Transaction {
	pool.mu.Lock()
	gasCount := new(big.Int)

	//转数据结构
	//type HeapByNonce map[crypto.CommonAddress]*nonceTxsHeap
	hbn := make(map[string]*nonceTxsHeap)
	func() {
		for accountName, list := range pool.pending {
			if !list.Empty() {
				txs := list.Flatten()
				newList := &nonceTxsHeap{}
				for _, tx := range txs {
					newList.Push(tx)
				}
				hbn[accountName] = newList
			}
		}
	}()
	pool.mu.Unlock()

	var retrunTxs []*chainTypes.Transaction
	for {
		for addr, list := range hbn {
			tx := heap.Pop(list).(*chainTypes.Transaction)
			if GasLimit.Cmp(new(big.Int).Add(tx.GasLimit(), gasCount)) >= 0 {
				retrunTxs = append(retrunTxs, tx)
			} else {
				goto END
			}

			if list.Len() == 0 {
				delete(hbn, addr)
			}
		}

		if len(hbn) <= 0 {
			goto END
		}
	}

END:
	return retrunTxs
}

func (pool *TransactionPool) Start(feed *event.Feed) {
	go pool.checkUpdate()
	pool.eventNewBlockSub = feed.Subscribe(pool.newBlockChan)
}

func (pool *TransactionPool) Stop() {
	close(pool.quit)
	pool.eventNewBlockSub.Unsubscribe()
}

func (pool *TransactionPool) checkUpdate() {
	for {
		select {
		case addrList := <-pool.newBlockChan:
			pool.adjust(addrList)
		case <-pool.quit:
			return
		}
	}
}

//已经被处理过NONCE都被清理出去
func (pool *TransactionPool) adjust(accountNameList []string) {
	for _, accountName := range accountNameList {
		// 获取数据库里面的nonce
		//根据nonce是否被处理，删除对应的交易
		nonce := pool.databaseApi.GetNonce(accountName, true)
		pool.mu.Lock()
		list, ok := pool.pending[accountName]
		if ok {
			txs := list.Forward(nonce)
			for _, tx := range txs {
				id, _ := tx.TxId()
				delete(pool.allTxs, id)

			}
		}
		pool.mu.Unlock()

		dlog.Warn("clear txpool",  "max tx.nonce:", nonce,"txpool tx count:", len(pool.allTxs))
	}
}

//获取总的交易个数，即获取地址对应的nonce
func (pool *TransactionPool) GetTransactionCount(accountName string) int64 {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	return pool.getTransactionCount(accountName)
}

func (pool *TransactionPool) getTransactionCount(accountName string) int64 {
	if nonce, ok := pool.pendingNonce[accountName]; ok {
		return nonce
	} else {
		nonce := pool.databaseApi.GetNonce(accountName, true)
		pool.pendingNonce[accountName] = nonce
		return nonce
	}
}