package service

import (
	"sync"
	"time"

	"github.com/drep-project/drep-chain/app"
	"github.com/drep-project/drep-chain/chain/params"
	"gopkg.in/urfave/cli.v1"

	"github.com/drep-project/binary"
	"github.com/drep-project/drep-chain/common"
	"github.com/drep-project/drep-chain/common/event"
	"github.com/drep-project/drep-chain/crypto"
	"github.com/drep-project/drep-chain/crypto/sha3"
	"github.com/drep-project/drep-chain/database"
	"github.com/drep-project/drep-chain/pkgs/evm"

	chainTypes "github.com/drep-project/drep-chain/chain/types"
	rpc2 "github.com/drep-project/drep-chain/pkgs/rpc"
)

var (
	rootChain           app.ChainIdType
	DefaultChainConfig = &chainTypes.ChainConfig{
		RemotePort:  55556,
		ChainId:     app.ChainIdType{},
		GenesisPK:   "0x03177b8e4ef31f4f801ce00260db1b04cc501287e828692a404fdbc46c7ad6ff26",
	}
	span = uint64(params.MaxGasLimit / 360)
)

type ChainService struct {
	RpcService      *rpc2.RpcService          `service:"rpc"`
	DatabaseService *database.DatabaseService `service:"database"`
	VmService       evm.Vm                    `service:"vm"`
	apis            []app.API

	stateProcessor *StateProcessor

	chainId app.ChainIdType

	lock          sync.RWMutex
	addBlockSync  sync.Mutex

	// These fields are related to handling of orphan blocks.  They are
	// protected by a combination of the chain lock and the orphan lock.
	orphanLock   sync.RWMutex
	orphans      map[crypto.Hash]*chainTypes.OrphanBlock
	prevOrphans  map[crypto.Hash][]*chainTypes.OrphanBlock
	oldestOrphan *chainTypes.OrphanBlock

	Index         *chainTypes.BlockIndex
	BestChain     *chainTypes.ChainView
	stateLock     sync.RWMutex
	StateSnapshot *ChainState

	Config       *chainTypes.ChainConfig
	genesisBlock *chainTypes.Block
	//Events related to sync blocks
	syncBlockEvent event.Feed

	//提供新块订阅
	NewBlockFeed    event.Feed
	DetachBlockFeed event.Feed
}

type ChainState struct {
	chainTypes.BestState
	db *database.Database
}

func (chainService *ChainService) ChainID() app.ChainIdType {
	return chainService.chainId
}

func (chainService *ChainService) Name() string {
	return "chain"
}

func (chainService *ChainService) Api() []app.API {
	return chainService.apis
}

func (chainService *ChainService) CommandFlags() ([]cli.Command, []cli.Flag) {
	return nil, []cli.Flag{}
}

func (chainService *ChainService) Init(executeContext *app.ExecuteContext) error {
	chainService.Config = DefaultChainConfig

	err := executeContext.UnmashalConfig(chainService.Name(), chainService.Config)
	if err != nil {
		return err
	}
	chainService.Index = chainTypes.NewBlockIndex()
	chainService.BestChain = chainTypes.NewChainView(nil)
	chainService.orphans = make(map[crypto.Hash]*chainTypes.OrphanBlock)
	chainService.prevOrphans = make(map[crypto.Hash][]*chainTypes.OrphanBlock)
	chainService.stateProcessor = NewStateProcessor(chainService)

	chainService.genesisBlock = chainService.GetGenisiBlock(chainService.Config.GenesisPK)
	hash := chainService.genesisBlock.Header.Hash()
	if !chainService.DatabaseService.HasBlock(hash) {
		chainService.genesisBlock, err = chainService.ProcessGenesisBlock(chainService.Config.GenesisPK)
		err = chainService.createChainState()
		if err != nil {
			return err
		}
	}
	chainService.InitStates()

	chainService.apis = []app.API{
		app.API{
			Namespace: "chain",
			Version:   "1.0",
			Service: &ChainApi{
				chainService: chainService,
				dbService:    chainService.DatabaseService,
			},
			Public: true,
		},
	}
	return nil
}

func (chainService *ChainService) Start(executeContext *app.ExecuteContext) error {
	return nil
}

func (chainService *ChainService) Stop(executeContext *app.ExecuteContext) error {
	return nil
}

func (chainService *ChainService) BlockExists(blockHash *crypto.Hash) bool {
	return chainService.Index.HaveBlock(blockHash)
}


func (chainService *ChainService) RootChain() app.ChainIdType {
	return rootChain
}

func (chainService *ChainService) SubscribeSyncBlockEvent(subchan chan event.SyncBlockEvent) event.Subscription {
	return chainService.syncBlockEvent.Subscribe(subchan)
}


func (chainService *ChainService) GetBlocksFrom(start, size uint64) ([]*chainTypes.Block, error) {
	blocks := []*chainTypes.Block{}
	for i := start; i < start+size; i++ {
		node := chainService.BestChain.NodeByHeight(i)
		if node == nil {
			continue
		}
		block, err := chainService.DatabaseService.GetBlock(node.Hash)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

func (chainService *ChainService) GetHighestBlock() (*chainTypes.Block, error) {
	heighestBlockBode := chainService.BestChain.Tip()
	block, err := chainService.DatabaseService.GetBlock(heighestBlockBode.Hash)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (chainService *ChainService) GetBlockByHash(hash *crypto.Hash) (*chainTypes.Block, error) {
	block, err := chainService.DatabaseService.GetBlock(hash)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (chainService *ChainService) GetBlockHeaderByHash(hash *crypto.Hash) (*chainTypes.BlockHeader, error) {
	blockNode, ok := chainService.Index.Index[*hash]
	if !ok {
		return nil, ErrBlockNotFound
	}
	blockHeader := blockNode.Header()
	return &blockHeader, nil
}

func (chainService *ChainService) GetHeader(hash crypto.Hash, number uint64) *chainTypes.BlockHeader {
	header, _ := chainService.GetBlockHeaderByHash(&hash)
	return header
}

func (chainService *ChainService) GetBlockByHeight(number uint64) (*chainTypes.Block, error) {
	blockNode := chainService.BestChain.NodeByHeight(number)
	return chainService.GetBlockByHash(blockNode.Hash)
}

func (chainService *ChainService) GetBlockHeaderByHeight(number uint64) (*chainTypes.BlockHeader, error) {
	blockNode := chainService.BestChain.NodeByHeight(number)
	if blockNode == nil {
		return nil, ErrBlockNotFound
	}
	header := blockNode.Header()
	return &header, nil
}

func (chainService *ChainService) getTxHashes(ts []*chainTypes.Transaction) ([][]byte, error) {
	txHashes := make([][]byte, len(ts))
	for i, tx := range ts {
		b, err := binary.Marshal(tx.Data)
		if err != nil {
			return nil, err
		}
		txHashes[i] = sha3.Keccak256(b)
	}
	return txHashes, nil
}

func (cs *ChainService) DeriveMerkleRoot(txs []*chainTypes.Transaction) []byte {
	if len(txs) == 0 {
		return []byte{}
	}
	ts, _ := cs.getTxHashes(txs)
	merkle := common.NewMerkle(ts)
	return merkle.Root.Hash
}

func (chainService *ChainService) createChainState() error {
	node := chainTypes.NewBlockNode(chainService.genesisBlock.Header, nil)
	node.Status = chainTypes.StatusDataStored | chainTypes.StatusValid
	chainService.BestChain.SetTip(node)

	// Add the new node to the index which is used for faster lookups.
	chainService.Index.AddNode(node)

	// Initialize the state related to the best block.  Since it is the
	// genesis block, use its timestamp for the median time.
	chainService.stateLock.Lock()
	chainService.StateSnapshot = &ChainState{
		BestState: *chainTypes.NewBestState(node, time.Unix(int64(node.TimeStamp), 0)),
		db:        chainService.DatabaseService.BeginTransaction(),
	}
	chainService.stateLock.Unlock()

	// Save the genesis block to the block index database.
	err := chainService.DatabaseService.PutBlockNode(node)
	if err != nil {
		return err
	}

	// Store the current best chain state into the database.
	chainService.stateLock.Lock()
	state := chainService.StateSnapshot.BestState
	chainService.stateLock.Unlock()
	err = chainService.DatabaseService.PutChainState(&state)
	if err != nil {
		return err
	}
	err = chainService.DatabaseService.PutBlock(chainService.genesisBlock)
	if err != nil {
		return err
	} else {
		return nil
	}
}

func (chainService *ChainService) GetCurrentState() *database.Database {
	chainService.stateLock.Lock()
	defer chainService.stateLock.Unlock()
	return  chainService.StateSnapshot.db

}