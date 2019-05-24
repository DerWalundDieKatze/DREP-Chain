package blockmgr

import (
	"fmt"
	"math/big"
	"math/rand"
	"path"
	"sync"

	"github.com/drep-project/drep-chain/chain/params"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/drep-project/drep-chain/app"
	"gopkg.in/urfave/cli.v1"

	"github.com/drep-project/drep-chain/chain/txpool"
	"github.com/drep-project/drep-chain/common/event"
	"github.com/drep-project/drep-chain/crypto"
	"github.com/drep-project/drep-chain/database"
	"github.com/drep-project/drep-chain/network/p2p"
	"github.com/drep-project/drep-chain/pkgs/evm"
	"github.com/drep-project/drep-chain/rpc"

	"github.com/drep-project/drep-chain/chain/service/chainservice"
	chainTypes "github.com/drep-project/drep-chain/chain/types"
	p2pService "github.com/drep-project/drep-chain/network/service"

	rpc2 "github.com/drep-project/drep-chain/pkgs/rpc"
	"github.com/drep-project/drep-chain/common"
	"time"
)

var (
	rootChain           app.ChainIdType
	DefaultOracleConfig = chainTypes.OracleConfig{
		Blocks:     20,
		Default:    big.NewInt(params.GWei).Uint64(),
		Percentile: 60,
		MaxPrice:   big.NewInt(500 * params.GWei).Uint64(),
	}
	DefaultChainConfig = &chainTypes.BlockMgr{
		GasPrice:    DefaultOracleConfig,
		JournalFile: "txpool/txs",
	}
	span = uint64(params.MaxGasLimit / 360)
)

type BlockMgr struct {
	ChainService    *chainservice.ChainService         `service:"chain"`
	RpcService      *rpc2.RpcService          `service:"rpc"`
	P2pServer       p2pService.P2P            `service:"p2p"`
	DatabaseService *database.DatabaseService `service:"database"`
	VmService       evm.Vm                    `service:"vm"`
	transactionPool *txpool.TransactionPool
	apis            []app.API

	lock sync.RWMutex

	Config *chainTypes.BlockMgr
	pid    *actor.PID
	//Events related to sync blocks
	syncBlockEvent event.Feed
	syncMut        sync.Mutex

	//从远端接收块头hash组
	headerHashCh chan []*syncHeaderHash

	//从远端接收到块
	blocksCh chan []*chainTypes.Block

	//所有需要同步的任务列表
	allTasks *heightSortedMap

	//正在同步中的任务列表，如果对应的块未到，会重新发布请求的
	pendingSyncTasks map[crypto.Hash]uint64
	taskTxsCh        chan tasksTxsSync

	//与此模块通信的所有Peer
	peersInfo map[string]*chainTypes.PeerInfo
	newPeerCh chan *chainTypes.PeerInfo

	gpo  *Oracle
	quit chan struct{}
}

type syncHeaderHash struct {
	headerHash *crypto.Hash
	height     uint64
}

func (blockMgr *BlockMgr) Name() string {
	return "blockmgr"
}

func (blockMgr *BlockMgr) Api() []app.API {
	return blockMgr.apis
}

func (blockMgr *BlockMgr) CommandFlags() ([]cli.Command, []cli.Flag) {
	return nil, []cli.Flag{}
}

func (blockMgr *BlockMgr) Init(executeContext *app.ExecuteContext) error {
	blockMgr.Config = DefaultChainConfig

	err := executeContext.UnmashalConfig(blockMgr.Name(), blockMgr.Config)
	if err != nil {
		return err
	}
	blockMgr.headerHashCh = make(chan []*syncHeaderHash)
	blockMgr.blocksCh = make(chan []*chainTypes.Block)
	blockMgr.allTasks = newHeightSortedMap()
	blockMgr.pendingSyncTasks = make(map[crypto.Hash]uint64)
	blockMgr.peersInfo = make(map[string]*chainTypes.PeerInfo)
	blockMgr.newPeerCh = make(chan *chainTypes.PeerInfo, maxLivePeer)
	blockMgr.taskTxsCh = make(chan tasksTxsSync, maxLivePeer)
	blockMgr.gpo = NewOracle(blockMgr.ChainService, blockMgr.Config.GasPrice)

	//TODO use disk db
	blockMgr.transactionPool = txpool.NewTransactionPool(blockMgr.ChainService.DatabaseService.Db(), path.Join(executeContext.CommonConfig.HomeDir, blockMgr.Config.JournalFile))

	blockMgr.P2pServer.AddProtocols([]p2p.Protocol{
		p2p.Protocol{
			Name:   "blockMgr",
			Length: chainTypes.NumberOfMsg,
			Run: func(peer *p2p.Peer, rw p2p.MsgReadWriter) error {
				if len(blockMgr.peersInfo) >= maxLivePeer {
					return ErrEnoughPeer
				}
				pi := chainTypes.NewPeerInfo(peer, rw)
				blockMgr.peersInfo[peer.IP()] = pi
				defer delete(blockMgr.peersInfo, peer.IP())
				return blockMgr.receiveMsg(pi, rw)
			},
		},
	})

	blockMgr.apis = []app.API{
		app.API{
			Namespace: "blockmgr",
			Version:   "1.0",
			Service: &BlockMgrApi{
				blockMgr:  blockMgr,
				dbService: blockMgr.DatabaseService,
			},
			Public: true,
		},
	}
	return nil
}

func (blockMgr *BlockMgr) Start(executeContext *app.ExecuteContext) error {
	blockMgr.transactionPool.Start(&blockMgr.ChainService.NewBlockFeed)
	go blockMgr.synchronise()
	go blockMgr.syncTxs()
	return nil
}

func (blockMgr *BlockMgr) Stop(executeContext *app.ExecuteContext) error {
	if blockMgr.quit != nil {
		close(blockMgr.quit)
	}

	return nil
}

func (blockMgr *BlockMgr) Attach() (*rpc.Client, error) {
	blockMgr.lock.RLock()
	defer blockMgr.lock.RUnlock()

	return rpc.DialInProc(blockMgr.RpcService.IpcHandler), nil
}

func (blockMgr *BlockMgr) GetTransactionCount(addr *crypto.CommonAddress) uint64 {
	return blockMgr.transactionPool.GetTransactionCount(addr)
}

func (blockMgr *BlockMgr) SendTransaction(tx *chainTypes.Transaction, islocal bool) error {
	//TODO  use pool nonce
	from, err := tx.From()
	nonce := blockMgr.transactionPool.GetTransactionCount(from)
	if nonce > tx.Nonce() {
		return fmt.Errorf("error nounce db nonce:%d != %d", nonce, tx.Nonce())
	}
	err = blockMgr.VerifyTransaction(tx)

	if err != nil {
		return err
	}
	err = blockMgr.transactionPool.AddTransaction(tx, islocal)
	if err != nil {
		return err
	} else {
		blockMgr.BroadcastTx(chainTypes.MsgTypeTransaction, tx, true)
	}
	return nil
}

func (blockMgr *BlockMgr) BroadcastBlock(msgType int32, block *chainTypes.Block, isLocal bool) {
	for _, peer := range blockMgr.peersInfo {
		b := peer.KnownBlock(block)
		if !b {
			if !isLocal {
				//收到远端来的消息，仅仅广播给1/3的peer
				rd := rand.Intn(broadcastRatio)
				if rd > 1 {
					continue
				}
			}
			peer.MarkBlock(block)
			blockMgr.P2pServer.Send(peer.GetMsgRW(), uint64(msgType), block)
		}
	}
}

func (blockMgr *BlockMgr) BroadcastTx(msgType int32, tx *chainTypes.Transaction, isLocal bool) {
	go func() {
		for _, peer := range blockMgr.peersInfo {
			b := peer.KnownTx(tx)
			if !b {
				if !isLocal {
					//收到远端来的消息，仅仅广播给1/3的peer
					rd := rand.Intn(broadcastRatio)
					if rd > 1 {
						continue
					}
				}

				peer.MarkTx(tx)
				blockMgr.P2pServer.Send(peer.GetMsgRW(), uint64(msgType), []*chainTypes.Transaction{tx})
			}
		}
	}()
}

func (blockMgr *BlockMgr) GetPoolTransactions(addr *crypto.CommonAddress) []chainTypes.Transactions {
	return blockMgr.transactionPool.GetTransactions(addr)
}

func (blockMgr *BlockMgr) GetPoolMiniPendingNonce(addr *crypto.CommonAddress) uint64 {
	return blockMgr.transactionPool.GetMiniPendingNonce(addr)
}

func (blockMgr *BlockMgr) GenerateTransferTransaction(to  *crypto.CommonAddress, nonce uint64, amount, price, limit common.Big) chainTypes.Transaction {
	t := chainTypes.Transaction{
		Data: chainTypes.TransactionData {
			Version:   common.Version,
			Nonce:     nonce,
			ChainId:   blockMgr.ChainService.ChainID(),
			Type:      chainTypes.TxType(chainTypes.TransferType),
			To:        *to,
			Amount:    amount,
			GasPrice:  price,
			GasLimit:  limit,
			Timestamp: time.Now().Unix(),
			Data:      []byte{},
		},
	}
	return t
}
