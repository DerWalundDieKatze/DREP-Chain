package types

import (
	"github.com/drep-project/drep-chain/crypto"
	"time"
)

type BestState struct {
	Hash      crypto.Hash
	Height    int64
	MedianTime time.Time
}

// newBestState returns a new best stats instance for the given parameters.
func NewBestState(node *BlockNode, medianTime time.Time) *BestState {

	return &BestState{
		Hash:        *node.Hash,
		Height:      node.Height,
		MedianTime:  medianTime,
	}
}