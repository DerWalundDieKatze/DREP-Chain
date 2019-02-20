package service

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	chainTypes "github.com/drep-project/drep-chain/chain/types"
	p2pTypes "github.com/drep-project/drep-chain/network/types"
)

func (chainService *ChainService) Receive(context actor.Context) {
	var msg interface{}
	msg = context.Message()
	routeMsg, ok := context.Message().(*p2pTypes.RouteIn)
	if ok {
		msg = routeMsg.Detail
	}
	switch msg := msg.(type) {
		case *chainTypes.BlockReq:
			chainService.ProcessBlockReq(routeMsg.Peer, msg)
		case *chainTypes.BlockResp:
			go func() {
				for _, block := range msg.Blocks {
					chainService.ProcessBlock(block)
				}
			}()

		case *chainTypes.Transaction:
			/*
			transaction := msg
			id, _ := transaction.TxId()
			if store.ForwardedTransaction(id) {
				log.Debug("Forwarded this transaction ", "transaction", *transaction)
				return
			}
			// TODO backup nodes should not add
			if store.AddTransaction(transaction) {
				log.Debug("Succeed to add this transaction ", "transaction", *transaction)
				chainService.p2pServer.Broadcast(transaction)
				store.ForwardTransaction(id)
			} else {
				log.Debug("Fail to add this transaction ", "transaction", *transaction)
			}*/
		case *chainTypes.Block:
			/*
			block := msg
			if block.Header.Height <= database.GetMaxHeight() {
				return
			}
			id, _ := block.BlockHashHex()
			if store.ForwardedBlock(id) { // if forwarded, then processed. later this will be read from db
				log.Debug("Forwarded this block ", "block" ,*block)
				return
			}
			store.ForwardBlock(id)
			_, err := chainService.processBlock(block)
			if err != nil {
				//chainService.consensusEngine.OnNewHeightUpdate(block.Header.Height)
			}
			*/
		case *p2pTypes.PeerState:
			chainService.handlePeerState(routeMsg.Peer, msg)
		case *p2pTypes.ReqPeerState:
			chainService.handleReqPeerState(routeMsg.Peer, msg)
		}
}