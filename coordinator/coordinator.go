package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/niftynei/glightning/glightning"
	"github.com/rsbondi/multifund-join/multijoin"
	"github.com/rsbondi/multifund/funder"
	"github.com/rsbondi/multifund/rpc"
	"github.com/rsbondi/multifund/wallet"
)

const VERSION = "0.0.1-WIP"

var plugin *glightning.Plugin
var fundr *funder.Funder
var queue []funder.FundingInfo

func handleJoin(w http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var joinReq funder.FundingInfo
	err := decoder.Decode(&joinReq)
	var res *multijoin.JoinStartResponse
	if err != nil {
		log.Printf("unable to decode request: %s", err.Error())
		res = &multijoin.JoinStartResponse{
			Response: "",
			Error:    err.Error(),
		}
	} else {
		log.Printf("queuing request: %v", joinReq.Recipients)
		n := len(joinReq.Outputs)
		word := "channel"
		if n > 1 {
			word = word + "s"
		}
		res = &multijoin.JoinStartResponse{
			Response: fmt.Sprintf("successfuly queued to join funding for %d %s", n, word),
			Error:    "",
		}
	}

	queue = append(queue, joinReq)
	if len(queue) >= 2 {
		f := &funder.FundingInfo{
			Recipients: make([]*wallet.TxRecipient, 0),
			Utxos:      make([]wallet.UTXO, 0),
		}
		for _, q := range queue {
			f.Recipients = append(f.Recipients, q.Recipients...)
			f.Utxos = append(f.Utxos, q.Utxos...)
		}

		tx, err := wallet.CreateTransaction(f.Recipients, f.Utxos, fundr.BitcoinNet)
		if err != nil {
			log.Printf("no go: %s", err.Error())
		}
		log.Printf("tx from join: %s", tx.String())
	}

	// TODO: store this or proceed if threshold reached
	//       how to respond once threshold is reached
	//         does user need to be listening also, how to track?
	//         or web socket?  what if connection breaks?
	json.NewEncoder(w).Encode(res)
}

func main() {
	plugin = glightning.NewPlugin(onInit)
	fundr = &funder.Funder{}
	fundr.Lightning = glightning.NewLightning()
	queue = make([]funder.FundingInfo, 0)
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
