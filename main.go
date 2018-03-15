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
	"math/big"
	"net"
	"net/http"
	"os"
	"time"
)

var (
	logger         *log.Logger
	nonVerifiedTxs = make(map[[32]byte]*protocol.FundsTx)
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage bazo-multisig <multisig>")
	}

	logger = storage.InitLogger()

	go nonVerifiedTxStatus()

	listener()
}

func nonVerifiedTxStatus() {
	for {
		if len(nonVerifiedTxs) == 0 {
			logger.Println("No non verified transactions.")
		} else {
			logger.Println("Non verified transactions:")
		}
		for to, tx := range nonVerifiedTxs {
			fmt.Printf("%x: %x\n", to[:8], tx.Sig2[:8])
		}
		fmt.Println()

		time.Sleep(10 * time.Second)
	}
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
			logger.Print(err)
			continue
		}

		go serve(c)
	}
}

func serve(c net.Conn) {
	header, payload, err := p2p.RcvData(c)
	if err != nil {
		logger.Printf("Failed to handle incoming connection: %v\n", err)
		c.Close()
		return
	}

	if header.TypeID == p2p.FUNDSTX_BRDCST {
		var err error
		var tx *protocol.FundsTx
		var packet []byte

		if tx = tx.Decode(payload); tx == nil {
			err = errors.New("Tx decoding failed.")
		}

		txHash := tx.Hash()
		err = processTx(tx)

		if err != nil {
			logger.Printf("Processing tx %x failed: %v", txHash[:8], err)
			packet = p2p.BuildPacket(p2p.NOT_FOUND, []byte(fmt.Sprintf("Processing tx %x failed: %v", txHash[:8], err)))
		} else {
			packet = p2p.BuildPacket(p2p.TX_BRDCST_ACK, nil)
		}

		c.Write(packet)
	} else if header.TypeID == p2p.VERIFIEDTX_BRDCST {
		for _, data := range protocol.Decode(payload, protocol.FUNDSTX_SIZE) {
			var verifiedTx *protocol.FundsTx
			verifiedTx = verifiedTx.Decode(data)

			delete(nonVerifiedTxs, verifiedTx.Hash())
		}

		packet := p2p.BuildPacket(p2p.TX_BRDCST_ACK, nil)
		c.Write(packet)
	} else if header.TypeID == p2p.FUNDSTX_REQ {
		var addressHash [32]byte
		copy(addressHash[:], payload[:32])

		var nonVerifiedTxsTo [][]byte

		for _, tx := range nonVerifiedTxs {
			if tx.Sig2 != [64]byte{} && (tx.From == addressHash || tx.To == addressHash) {
				nonVerifiedTxsTo = append(nonVerifiedTxsTo, tx.Encode()[:])
			}
		}

		packet := p2p.BuildPacket(p2p.FUNDSTX_RES, protocol.Encode(nonVerifiedTxsTo, protocol.FUNDSTX_SIZE))
		c.Write(packet)
	}

	c.Close()
}

func processTx(tx *protocol.FundsTx) (err error) {
	acc, err := reqAccount(tx.From)
	if err != nil {
		return err
	}

	if err := verify(tx, acc); err == nil {
		if err := signTx(tx); err != nil {
			return err
		}

		for _, nonVerifiedTx := range nonVerifiedTxs {
			if nonVerifiedTx.To == tx.To && nonVerifiedTx.Sig2 == [64]byte{} {
				delete(nonVerifiedTxs, nonVerifiedTx.Hash())
			}
		}

		nonVerifiedTxs[tx.Hash()] = tx

		if err := sendTx(tx); err != nil {
			return err
		}
	} else {
		nonVerifiedTxs[tx.Hash()] = tx

		return err
	}

	return nil
}

func verify(tx *protocol.FundsTx, acc *client.Account) error {
	if !acc.IsRoot {

		//Use signed int, since otherwise it might happen that a negative balance will become positive
		var tmpBalance int64
		tmpBalance = int64(acc.Balance)

		for _, nonVerifiedTx := range nonVerifiedTxs {
			if nonVerifiedTx.Sig2 != [64]byte{} {
				if nonVerifiedTx.From == tx.From {
					tmpBalance -= int64(tx.Amount)
				}
				if nonVerifiedTx.To == tx.From {
					tmpBalance += int64(tx.Amount)
				}
			}
		}

		if acc.TxCnt != tx.TxCnt {
			return errors.New(fmt.Sprintf("Sender txCnt does not match: %v (tx.txCnt) vs. %v (state txCnt).", tx.TxCnt, acc.TxCnt))
		}

		if tmpBalance < int64(tx.Amount)+int64(tx.Fee) {
			return errors.New(fmt.Sprintf("Not enough balance: %v (tx.amount) vs. %v (state balance).", tx.Amount, acc.Balance))
		}

		if err := verifySig1(acc, tx); err != nil {
			return errors.New(fmt.Sprintf("Sender's signature (Sig1) failed."))
		}
	}

	return nil
}

func verifySig1(acc *client.Account, tx *protocol.FundsTx) error {
	runes := []rune(acc.AddressString)
	pub1 := string(runes[:64])
	pub2 := string(runes[64:])

	pubKey, _ := storage.GetPubKeyFromString(pub1, pub2)

	r, s := new(big.Int), new(big.Int)
	r.SetBytes(tx.Sig1[:32])
	s.SetBytes(tx.Sig1[32:])

	txHash := tx.Hash()

	if !ecdsa.Verify(&pubKey, txHash[:], r, s) {
		return errors.New("Tx verification failed.")
	}

	return nil
}

func reqAccount(addressHash [32]byte) (acc *client.Account, err error) {
	response, err := http.Get("http://" + client.LIGHT_CLIENT_SERVER + "/account/" + hex.EncodeToString(addressHash[:]))
	if err != nil {
		return nil, errors.New(fmt.Sprintf("The HTTP request failed with error %v", err))
	}

	data, _ := ioutil.ReadAll(response.Body)

	var contents []REST.Content
	content := REST.Content{"account", &acc}
	contents = append(contents, content)

	jsonResponse := REST.JsonResponse{Content: contents}
	json.Unmarshal([]byte(data), &jsonResponse)

	if acc == nil || !acc.IsCreated {
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
		return errors.New(fmt.Sprintf("The HTTP request failed with error %v", err))
	}

	data, _ := ioutil.ReadAll(response.Body)
	json.Unmarshal([]byte(data), &jsonResponse)

	if jsonResponse.Code != 200 {
		return errors.New(fmt.Sprintf("Could not send tx. Error code: %v", jsonResponse.Code))
	}

	return nil
}
