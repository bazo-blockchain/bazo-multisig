package network

import (
	"github.com/bazo-blockchain/bazo-miner/protocol"
)

func verifiedTxsBrdcst(p *peer, payload []byte) {
	for _, data := range protocol.Decode(payload, protocol.FUNDSTX_SIZE) {
		var verifiedTx *protocol.FundsTx
		verifiedTx = verifiedTx.Decode(data)

		delete(NonVerifiedTxs, verifiedTx.Hash())
	}
}
