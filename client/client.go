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

func main() {
	plugin = glightning.NewPlugin(onInit)
	fundr = &funder.Funder{}
	fundr.Lightning = glightning.NewLightning()
	fundr.Wallettype = wallet.WALLET_INTERNAL

	rpc.Init(fundr.Lightning)
	registerMethods(plugin)

	err := plugin.Start(os.Stdin, os.Stdout)
	if err != nil {
		log.Fatal(err)
	}
}

func onInit(plugin *glightning.Plugin, options map[string]string, config *glightning.Config) {
	log.Printf("versiion: %s initialized.", VERSION)
	fundr.Lightningdir = config.LightningDir
	options["rpc-file"] = fmt.Sprintf("%s/%s", config.LightningDir, config.RpcFile)
	log.Printf("reading from rpc file: %s", options["rpc-file"])
	fundr.Lightning.StartUp(config.RpcFile, config.LightningDir)
	fundr.Bitcoin = wallet.NewBitcoinWallet() // TODO: do something different in other packege for fee estimate, this will probably change but bitcoin wallet should not be needed here

	cfg, err := rpc.ListConfigs()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("bitcoin network(from listconfig): %s", cfg.Network)

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

	fundr.Internal = fundr.InternalWallet()
}

// fund_multi [{"id":"0265b6...", "satoshi": 20000, "announce":true}, {id, satoshi, announce}...]
func registerMethods(p *glightning.Plugin) {
	multi := glightning.NewRpcMethod(&MultiChannelJoin{}, `Open multiple channels in single transaction`)
	multi.LongDesc = JoinMultiDescription
	multi.Usage = "channels"
	p.RegisterMethod(multi)
	log.Printf("method registered: %s", multi.Description())
}

// I think I need options for server address ore something?
