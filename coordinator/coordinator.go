package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

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
var mixid = 1
var mix map[string]wallet.Transaction

func handleJoin(w http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var joinReq funder.FundingInfo
	err := decoder.Decode(&joinReq)
	var res *multijoin.JoinStartResponse
	if err != nil {
		log.Printf("unable to decode request: %s", err.Error())
		data := &multijoin.JoinStartResponseData{}
		res = &multijoin.JoinStartResponse{
			Response: data,
			Error:    err.Error(),
		}
	} else {
		log.Printf("queuing request: %v", joinReq.Recipients)
		n := len(joinReq.Outputs)
		word := "channel"
		if n > 1 {
			word = word + "s"
		}
		data := &multijoin.JoinStartResponseData{
			Message: fmt.Sprintf("successfuly queued to join funding for %d %s", n, word),
			Id:      mixid,
		}
		res = &multijoin.JoinStartResponse{
			Response: data,
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
		mix[string(mixid)] = tx
		mixid++
	}

	// TODO: store this or proceed if threshold reached
	//       how to respond once threshold is reached
	//         does user need to be listening also, how to track?
	//         or web socket?  what if connection breaks?
	json.NewEncoder(w).Encode(res)
}

func handleStatus(w http.ResponseWriter, req *http.Request) {
	id := req.URL.Path[len("/status/"):]
	var res *multijoin.JoinStatusResponse
	if tx, ok := mix[id]; ok {
		res = &multijoin.JoinStatusResponse{
			Tx:    &tx.Unsigned,
			Error: "",
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	mid, err := strconv.ParseInt(id, 10, 32)
	if err != nil {
		res = &multijoin.JoinStatusResponse{
			Tx:    nil,
			Error: err.Error(),
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	if int(mid) > mixid {
		res = &multijoin.JoinStatusResponse{
			Tx:    nil,
			Error: "Invalid mix id",
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	res = &multijoin.JoinStatusResponse{
		Tx:    nil,
		Error: "",
	}
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
	http.HandleFunc("/status/", handleStatus)

	log.Printf("listening: %s", options["multi-join-port"])

	log.Fatal(http.ListenAndServe(":"+options["multi-join-port"], nil))
}
