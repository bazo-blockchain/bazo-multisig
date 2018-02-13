package main

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mchetelat/bazo_client/client"
	"github.com/mchetelat/bazo_miner/p2p"
	"github.com/mchetelat/bazo_miner/protocol"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
)

var (
	logger   *log.Logger
	fundsTxs = make(map[[32]byte]*protocol.FundsTx)
)

func main() {
	logger = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)

	listener()
}

func listener() {
	listener, err := net.Listen("tcp", ":8002")
	if err != nil {
		logger.Fatal(err)
	}

	for {
		c, err := listener.Accept()
		if err != nil {
			logger.Println(err)
			continue
		}

		go serve(c)
	}
}

func serve(c net.Conn) {
	header, payload, err := rcvData(c)
	if err != nil {
		logger.Printf("Failed to handle incoming connection: %v\n", err)
		return
	}

	if header.TypeID == p2p.FUNDSTX_BRDCST {
		var fundsTx *protocol.FundsTx

		if fundsTx = fundsTx.Decode(payload); fundsTx == nil {
			return
		}

		packet := p2p.BuildPacket(p2p.TX_BRDCST_ACK, nil)
		c.Write(packet)

		processTx(fundsTx)
	}

	c.Close()
}

func processTx(fundsTx *protocol.FundsTx) {
	//if fundsTx.Header != [32]byte{} {
	balance, err := reqBalance(fundsTx.From)
	if err != nil {
	}

	if checkSolvency(fundsTx.From, fundsTx.Amount, balance) {
		fundsTxs[fundsTx.Header] = fundsTx
		multisignTx(fundsTx)
		//sendTx()
	}
	//}
}

func checkSolvency(pubKeyHash [32]byte, amount uint64, balance uint64) bool {
	solvent := false
	tmpBalance := balance

	for _, fundsTx := range fundsTxs {
		if fundsTx.From == pubKeyHash {
			tmpBalance -= fundsTx.Amount
		}
		if fundsTx.To == pubKeyHash {
			tmpBalance += fundsTx.Amount
		}
	}

	if tmpBalance >= amount {
		solvent = true
	}

	return solvent
}

func reqBalance(pubKeyHash [32]byte) (uint64, error) {
	response, err := http.Get("http://127.0.0.1:8001/account/" + hex.EncodeToString(pubKeyHash[:]))
	if err != nil {
		return 0, errors.New(fmt.Sprintf("The HTTP request failed with error %s\n", err))
	}

	data, _ := ioutil.ReadAll(response.Body)
	var acc client.Account
	json.Unmarshal([]byte(data), &acc)

	return acc.Balance, nil
}

func multisignTx(fundsTx *protocol.FundsTx) {

}

func sendTx(fundsTx *protocol.FundsTx) {

}

func rcvData(c net.Conn) (header *p2p.Header, payload []byte, err error) {
	reader := bufio.NewReader(c)
	header, err = p2p.ReadHeader(reader)
	if err != nil {
		c.Close()
		return nil, nil, errors.New(fmt.Sprintf("Connection to aborted: (%v)\n", err))
	}
	payload = make([]byte, header.Len)

	for cnt := 0; cnt < int(header.Len); cnt++ {
		payload[cnt], err = reader.ReadByte()
		if err != nil {
			c.Close()
			return nil, nil, errors.New(fmt.Sprintf("Connection to aborted: %v\n", err))
		}
	}

	return header, payload, nil
}
