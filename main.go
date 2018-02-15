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
	logger  *log.Logger
	openTxs = make(map[[32]byte]*protocol.FundsTx)
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage bazo-multisig <keyfile>")
	}

	logger = storage.InitLogger()

	listener()
}

func listener() {
	listener, err := net.Listen("tcp", client.MULTISIG_SERVER_PORT)
	if err != nil {
		logger.Fatal(err)
	} else {
		logger.Printf("Listening on port %v", client.MULTISIG_SERVER_PORT)
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
		c.Close()
		return
	}

	if header.TypeID == p2p.FUNDSTX_BRDCST {
		var tx *protocol.FundsTx

		if tx = tx.Decode(payload); tx == nil {
			c.Close()
			return
		}

		packet := p2p.BuildPacket(p2p.TX_BRDCST_ACK, nil)
		c.Write(packet)

		txHash := tx.Hash()
		if err := processTx(tx); err != nil {
			logger.Printf("Processing tx %x failed: %v", txHash[:8], err)
		}
	}

	c.Close()
}

func processTx(tx *protocol.FundsTx) (err error) {
	acc, err := reqAccount(tx.From)
	if err != nil {
		return err
	}

	if checkSolvency(tx, acc) {
		openTxs[tx.Hash()] = tx

		if err := signTx(tx); err != nil {
			return err
		}

		if err := sendTx(tx); err != nil {
			return err
		}
	}

	return nil
}

func checkSolvency(tx *protocol.FundsTx, acc *client.Account) bool {
	//Use signed int, since otherwise it might happen that a negative balance will become positive
	var tmpBalance int64

	solvent := false
	tmpBalance = int64(acc.Balance)

	if !acc.IsRoot {
		for _, openTx := range openTxs {
			if openTx.From == tx.From {
				tmpBalance -= int64(tx.Amount)
			}
			if openTx.To == tx.From {
				tmpBalance += int64(tx.Amount)
			}
		}
	}

	if tmpBalance >= int64(tx.Amount)+int64(tx.Fee) || acc.IsRoot {
		solvent = true
	} else {
		logger.Printf("Account %x is not solvent", tx.From[:8])
	}

	return solvent
}

func reqAccount(addressHash [32]byte) (acc *client.Account, err error) {
	response, err := http.Get("http://" + client.LIGHT_CLIENT_SERVER + "/account/" + hex.EncodeToString(addressHash[:]))
	if err != nil {
		return nil, errors.New(fmt.Sprintf("The HTTP request failed with error %s", err))
	}

	data, _ := ioutil.ReadAll(response.Body)
	json.Unmarshal([]byte(data), &acc)

	if !acc.IsCreated {
		return nil, errors.New(fmt.Sprintf("Account %x not found", addressHash[:8]))
	}

	return acc, nil
}

func signTx(tx *protocol.FundsTx) error {
	_, privKey, _ := storage.ExtractKeyFromFile(os.Args[1])

	txHash := tx.Hash()
	r, s, err := ecdsa.Sign(rand.Reader, &privKey, txHash[:])
	if err != nil {
		return errors.New(fmt.Sprintf("Could not sign tx: %v", err))
	}

	copy(tx.Sig2[32-len(r.Bytes()):32], r.Bytes())
	copy(tx.Sig2[64-len(s.Bytes()):], s.Bytes())

	return nil
}

func sendTx(tx *protocol.FundsTx) error {
	var jsonResponse REST.JsonResponse
	jsonValue, _ := json.Marshal(jsonResponse)

	txHash := tx.Hash()
	response, err := http.Post("http://"+client.LIGHT_CLIENT_SERVER+"/sendFundsTx/"+hex.EncodeToString(txHash[:])+"/"+hex.EncodeToString(tx.Sig2[:]), "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		return errors.New(fmt.Sprintf("The HTTP request failed with error %s", err))
	}

	data, _ := ioutil.ReadAll(response.Body)
	json.Unmarshal([]byte(data), &jsonResponse)

	if jsonResponse.Code != 200 {
		return errors.New(fmt.Sprintf("Could not send tx. Error code: %s", jsonResponse.Code))
	}

	return nil
}
