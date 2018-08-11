package main

import (
	"github.com/bazo-blockchain/bazo-multisig/multisig"
	"github.com/bazo-blockchain/bazo-multisig/network"
	"github.com/bazo-blockchain/bazo-multisig/utils"
	"log"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage bazo-multisig <multisig>")
	}

	utils.Config = utils.LoadConfiguration()
	network.Init()
	multisig.Init()
}
