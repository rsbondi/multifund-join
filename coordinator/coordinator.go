package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

func handleJoin(w http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var joinReq funder.FundingInfo
	err := decoder.Decode(&joinReq)
	log.Printf("request recieved: %v", joinReq.Recipients[1].Address)
	if err != nil {
		log.Printf("unable to decode join request: %s", err.Error())
	}

	// TODO: store this or proceed if threshold reached
	//       how to respond once threshold is reached
	//         does user need to be listening also, how to track?
	//         or web socket?  what if connection breaks?
	w.Write([]byte("successfuly queued to join channel funding"))
}

func main() {
	plugin = glightning.NewPlugin(onInit)
	fundr = &funder.Funder{}
	fundr.Lightning = glightning.NewLightning()
	rpc.Init(fundr.Lightning)

	registerOptions(plugin)

	err := plugin.Start(os.Stdin, os.Stdout)
	if err != nil {
		log.Fatal(err)
	}
}

func registerOptions(p *glightning.Plugin) {
	p.RegisterOption(glightning.NewOption("multi-join-port", "where to listen", "9775"))
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

	http.HandleFunc("/join", handleJoin)

	log.Printf("listening: %s", options["multi-join-port"])

	log.Fatal(http.ListenAndServe(":"+options["multi-join-port"], nil))
}
