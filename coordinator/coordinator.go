package main

import (
	"fmt"
	"log"
	"os"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/niftynei/glightning/glightning"
	"github.com/rsbondi/multifund/funder"
	"github.com/rsbondi/multifund/rpc"
	"github.com/rsbondi/multifund/wallet"
)

const VERSION = "0.0.1-WIP"

var plugin *glightning.Plugin
var fundr *funder.Funder

func handleAddresses(info funder.FundingInfo) {

}

func main() {
	plugin = glightning.NewPlugin(onInit)
	fundr = &funder.Funder{}
	fundr.Lightning = glightning.NewLightning()
	rpc.Init(fundr.Lightning)

	err := plugin.Start(os.Stdin, os.Stdout)
	if err != nil {
		log.Fatal(err)
	}
}

func onInit(plugin *glightning.Plugin, options map[string]string, config *glightning.Config) {
	log.Printf("versiion: %s initialized.", VERSION)
	fundr.Lightningdir = config.LightningDir
	options["rpc-file"] = fmt.Sprintf("%s/%s", config.LightningDir, config.RpcFile)
	fundr.Lightning.StartUp(config.RpcFile, config.LightningDir)
	fundr.Bitcoin = wallet.NewBitcoinWallet()

	cfg, err := rpc.ListConfigs()
	if err != nil {
		log.Fatal(err)
	}

	switch cfg.Network {
	case "bitcoin":
		fundr.BitcoinNet = &chaincfg.MainNetParams
	case "regtest":
		fundr.BitcoinNet = &chaincfg.RegressionNetParams
	case "signet":
		panic("unsupported network")
	default:
		fundr.BitcoinNet = &chaincfg.TestNet3Params
	}

	// set up routes here?
}
