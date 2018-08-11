package network

import "github.com/bazo-blockchain/bazo-miner/p2p"

func neighborReq() {
	p := peers.getRandomPeer()
	if p == nil {
		logger.Print("Could not fetch a random peer.\n")
		return
	}

	packet := p2p.BuildPacket(p2p.NEIGHBOR_REQ, nil)
	sendData(p, packet)
}
