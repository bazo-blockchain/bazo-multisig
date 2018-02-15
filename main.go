package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bazo-blockchain/bazo-client/REST"
	"github.com/bazo-blockchain/bazo-client/client"
	"github.com/bazo-blockchain/bazo-miner/p2p"
	"github.com/bazo-blockchain/bazo-miner/protocol"
	"github.com/bazo-blockchain/bazo-miner/storage"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
)

var (
	logger *log.Logger
	openTx = make(map[[32]byte]*protocol.FundsTx)
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage bazo-multisig <keyfile>")
	}

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
	header, payload, err := client.RcvData(c)
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

func processTx(tx *protocol.FundsTx) {
	acc, err := reqAccount(tx.From)
	if err != nil {
	}

	if checkSolvency(tx.From, tx.Amount, acc) {
		openTx[tx.Hash()] = tx

		signTx(tx)

		sendTx(tx)
	}
}

func checkSolvency(pubKeyHash [32]byte, amount uint64, acc client.Account) bool {
	solvent := false
	tmpBalance := acc.Balance

	if !acc.IsRoot {
		for _, fundsTx := range openTx {
			if fundsTx.From == pubKeyHash {
				tmpBalance -= fundsTx.Amount
			}
			if fundsTx.To == pubKeyHash {
				tmpBalance += fundsTx.Amount
			}
		}
	}

	if tmpBalance >= amount || acc.IsRoot {
		solvent = true
	}

	return solvent
}

func reqAccount(pubKeyHash [32]byte) (acc client.Account, err error) {
	response, err := http.Get("http://" + client.LIGHT_CLIENT_SERVER + "/account/" + hex.EncodeToString(pubKeyHash[:]))
	if err != nil {
		return acc, errors.New(fmt.Sprintf("The HTTP request failed with error %s\n", err))
	}

	data, _ := ioutil.ReadAll(response.Body)
	json.Unmarshal([]byte(data), &acc)

	return acc, nil
}

func signTx(tx *protocol.FundsTx) {
	_, privKey, _ := storage.ExtractKeyFromFile(os.Args[1])

	txHash := tx.Hash()
	r, s, _ := ecdsa.Sign(rand.Reader, &privKey, txHash[:])

	copy(tx.Sig2[32-len(r.Bytes()):32], r.Bytes())
	copy(tx.Sig2[64-len(s.Bytes()):], s.Bytes())
}

func sendTx(tx *protocol.FundsTx) error {
	var jsonResponse REST.JsonResponse
	jsonValue, _ := json.Marshal(jsonResponse)

	txHash := tx.Hash()
	response, err := http.Post("http://"+client.LIGHT_CLIENT_SERVER+"/sendFundsTx/"+hex.EncodeToString(txHash[:])+"/"+hex.EncodeToString(tx.Sig2[:]), "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		return errors.New(fmt.Sprintf("The HTTP request failed with error %s\n", err))
	}

	data, _ := ioutil.ReadAll(response.Body)
	json.Unmarshal([]byte(data), &jsonResponse)

	fmt.Printf("%v\n", jsonResponse)

	return nil
}
