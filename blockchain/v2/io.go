package v2

import (
	"fmt"

	bc "github.com/tendermint/tendermint/blockchain"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

type iIO interface {
	sendBlockRequest(peerID p2p.ID, height int64) error
	sendBlockToPeer(block *types.Block, peerID p2p.ID) error
	sendBlockNotFound(height int64, peerID p2p.ID) error
	sendStatusResponse(height int64, peerID p2p.ID) error

	broadcastStatusRequest(base int64, height int64) error

	trySwitchToConsensus(state state.State, blocksSynced int)
}

type switchIO struct {
	sw *p2p.Switch
}

func newSwitchIo(sw *p2p.Switch) *switchIO {
	return &switchIO{
		sw: sw,
	}
}

const (
	// BlockchainChannel is a channel for blocks and status updates (`BlockStore` height)
	BlockchainChannel = byte(0x40)
)

type consensusReactor interface {
	// for when we switch from blockchain reactor and fast sync to
	// the consensus machine
	SwitchToConsensus(state.State, int)
}

func (sio *switchIO) sendBlockRequest(peerID p2p.ID, height int64) error {
	peer := sio.sw.Peers().Get(peerID)
	if peer == nil {
		return fmt.Errorf("peer not found")
	}
	bm, err := bc.MsgToProto(&bc.BlockRequestMessage{Height: height})
	if err != nil {
		return err
	}
	msgBytes, err := bm.Marshal()
	if err != nil {
		return err
	}
	queued := peer.TrySend(BlockchainChannel, msgBytes)
	if !queued {
		return fmt.Errorf("send queue full")
	}
	return nil
}

func (sio *switchIO) sendStatusResponse(height int64, peerID p2p.ID) error {
	peer := sio.sw.Peers().Get(peerID)
	if peer == nil {
		return fmt.Errorf("peer not found")
	}
	bm, err := bc.MsgToProto(&bc.StatusResponseMessage{Height: height})
	if err != nil {
		return err
	}
	msgBytes, err := bm.Marshal()
	if err != nil {
		return err
	}

	if queued := peer.TrySend(BlockchainChannel, msgBytes); !queued {
		return fmt.Errorf("peer queue full")
	}

	return nil
}

func (sio *switchIO) sendBlockToPeer(block *types.Block, peerID p2p.ID) error {
	peer := sio.sw.Peers().Get(peerID)
	if peer == nil {
		return fmt.Errorf("peer not found")
	}
	if block == nil {
		panic("trying to send nil block")
	}
	bm, err := bc.MsgToProto(&bc.BlockResponseMessage{Block: block})
	if err != nil {
		return err
	}
	msgBytes, err := bm.Marshal()
	if err != nil {
		return err
	}
	if queued := peer.TrySend(BlockchainChannel, msgBytes); !queued {
		return fmt.Errorf("peer queue full")
	}

	return nil
}

func (sio *switchIO) sendBlockNotFound(height int64, peerID p2p.ID) error {
	peer := sio.sw.Peers().Get(peerID)
	if peer == nil {
		return fmt.Errorf("peer not found")
	}
	bm, err := bc.MsgToProto(&bc.NoBlockResponseMessage{Height: height})
	if err != nil {
		return err
	}
	msgBytes, err := bm.Marshal()
	if err != nil {
		return err
	}

	if queued := peer.TrySend(BlockchainChannel, msgBytes); !queued {
		return fmt.Errorf("peer queue full")
	}

	return nil
}

func (sio *switchIO) trySwitchToConsensus(state state.State, blocksSynced int) {
	conR, ok := sio.sw.Reactor("CONSENSUS").(consensusReactor)
	if ok {
		conR.SwitchToConsensus(state, blocksSynced)
	}
}

func (sio *switchIO) broadcastStatusRequest(base, height int64) error {
	bm, err := bc.MsgToProto(&bc.StatusRequestMessage{Base: base, Height: height})
	if err != nil {
		return err
	}
	msgBytes, err := bm.Marshal()
	if err != nil {
		panic(err)
	}
	// XXX: maybe we should use an io specific peer list here
	sio.sw.Broadcast(BlockchainChannel, msgBytes)

	return nil
}
