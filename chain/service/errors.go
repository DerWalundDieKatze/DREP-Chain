package service

import (
	"errors"
)

var (
	ErrInvalidateTimestamp       = errors.New("timestamp equals parent's")
	ErrInvalidateBlockNumber     = errors.New("invalid block number")
	ErrBlockNotFound             = errors.New("block not exist")
	ErrTxIndexOutOfRange         = errors.New("tx index out of range")
	ErrReachGasLimit             = errors.New("gas limit reached")
	ErrOverFlowMaxMsgSize        = errors.New("msg exceed max size")
	ErrInvalidateBlockMultisig   = errors.New("verify multisig error")
	ErrUnsupportTxType           = errors.New("not support transaction type")
	ErrNegativeAmount            = errors.New("negative amount in tx")
	ErrExceedGasLimit            = errors.New("gas limit in tx has exceed block limit")
	ErrNonceTooHigh              = errors.New("nonce too high")
	ErrNonceTooLow               = errors.New("nonce too low")
	ErrTxPool                    = errors.New("transaction pool full")
	ErrEnoughPeer                = errors.New("peer exceed max peers")
	ErrNotContinueHeader         = errors.New("non contiguous header")
	ErrFindAncesstorTimeout      = errors.New("findAncestor timeout")
	ErrGetHeaderHashTimeout      = errors.New("get header hash timeout")
	ErrGetBlockTimeout           = errors.New("fetch blocks timeout")
	ErrReqStateTimeout           = errors.New("req state timeout")
	ErrInitStateFail             = errors.New("initChainState")
	ErrNotMathcedStateRoot       = errors.New("state root not matched")
	ErrGasUsed   				 = errors.New("gas used not matched")
	ErrDecodeMsg                 = errors.New("fail to decode p2p msg")
	ErrReadPeerMsg               = errors.New("fail to read peer msg")
	ErrMsgType                   = errors.New("not expected msg type")
	errBlockExsist               = errors.New("already have block")
	errBalance                   = errors.New("not enough balance")
	errGas                       = errors.New("not enough gas")
	errInsufficientBalanceForGas = errors.New("insufficient balance to pay for gas")
	errOrphanBlockExsist         = errors.New("already have block (orphan)")
)
