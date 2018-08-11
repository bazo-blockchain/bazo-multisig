package network

import (
	"github.com/bazo-blockchain/bazo-miner/p2p"
	"github.com/bazo-blockchain/bazo-miner/protocol"
)

var (
	iplistChan = make(chan string, p2p.MIN_MINERS)

	NonVerifiedTxs = make(map[[32]byte]*protocol.FundsTx)
)

func processIncomingMsg(p *peer, header *p2p.Header, payload []byte) {
	switch header.TypeID {
	//BROADCAST
	case p2p.VERIFIEDTX_BRDCST:
		verifiedTxsBrdcst(p, payload)
		//RESULTS
	case p2p.NEIGHBOR_RES:
	}
}
