package network

import (
	"github.com/bazo-blockchain/bazo-miner/protocol"
	"github.com/bazo-blockchain/bazo-miner/p2p"
	"strconv"
	"encoding/binary"
)

func verifiedTxsBrdcst(p *peer, payload []byte) {
	for _, data := range protocol.Decode(payload, protocol.FUNDSTX_SIZE) {
		var verifiedTx *protocol.FundsTx
		verifiedTx = verifiedTx.Decode(data)

		delete(NonVerifiedTxs, verifiedTx.Hash())
	}
}

func processNeighborRes(p *peer, payload []byte) {

	//Parse the incoming ipv4 addresses.
	ipportList := _processNeighborRes(payload)

	for _, ipportIter := range ipportList {
		logger.Printf("IP/Port received: %v\n", ipportIter)
		//iplistChan is a buffered channel to handle ips asynchronously.
		iplistChan <- ipportIter
	}
}

//Split the processNeighborRes function in two for cleaner testing.
func _processNeighborRes(payload []byte) (ipportList []string) {
	index := 0
	for cnt := 0; cnt < len(payload)/(p2p.IPV4ADDR_SIZE+p2p.PORT_SIZE); cnt++ {
		var addr string
		for singleAddr := index; singleAddr < index+p2p.IPV4ADDR_SIZE; singleAddr++ {
			tmp := int(payload[singleAddr])
			addr += strconv.Itoa(tmp)
			addr += "."
		}
		//Remove trailing dot.
		addr = addr[:len(addr)-1]
		addr += ":"
		//Extract port number.
		addr += strconv.Itoa(int(binary.BigEndian.Uint16(payload[index+4 : index+6])))

		ipportList = append(ipportList, addr)
		index += p2p.IPV4ADDR_SIZE + p2p.PORT_SIZE
	}
	return ipportList
}
