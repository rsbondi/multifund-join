package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	"github.com/niftynei/glightning/glightning"
	"github.com/rsbondi/multifund-join/multijoin"
	"github.com/rsbondi/multifund/funder"
	"github.com/rsbondi/multifund/wallet"
)

const VERSION = "0.0.1-WIP"
const N_PARTICIPANTS = 2

var plugin *glightning.Plugin
var fundr *funder.Funder

var queue map[int]JoinQueue
var mixid = 1

func handleJoin(w http.ResponseWriter, req *http.Request) {
	if _, ok := queue[int(mixid)]; !ok {
		log.Println("new queue")
		queue[int(mixid)] = NewJoinQueue()
	}
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
		pid := queue[mixid].Add(joinReq)
		data := &multijoin.JoinStartResponseData{
			Message: fmt.Sprintf("successfuly queued to join funding for join: %d participant: %d", mixid, pid),
			Id:      mixid,
			Pid:     pid,
		}
		res = &multijoin.JoinStartResponse{
			Response: data,
			Error:    "",
		}
	}

	if len(queue[mixid].Participants) >= N_PARTICIPANTS {
		f := &funder.FundingInfo{
			Recipients: make([]*wallet.TxRecipient, 0),
			Utxos:      make([]wallet.UTXO, 0),
		}
		for _, q := range queue[mixid].Participants {
			f.Recipients = append(f.Recipients, q.Recipients...)
			f.Utxos = append(f.Utxos, q.Utxos...)
		}

		tx, err := wallet.CreateTransaction(f.Recipients, f.Utxos, fundr.BitcoinNet)
		if err != nil {
			log.Printf("no go: %s", err.Error())
		}
		queue[mixid].SetTx(tx)
		mixid++
	}

	json.NewEncoder(w).Encode(res)
}

func handleStatus(w http.ResponseWriter, req *http.Request) {
	params := strings.Split(req.URL.Path[len("/status/"):], "/")
	id := params[0]
	participantid := params[1]
	var res *multijoin.JoinStatusResponse
	mid, err := strconv.ParseInt(id, 10, 32)
	m := int(mid)
	pid, err := strconv.ParseInt(participantid, 10, 32)
	p := int(pid)
	b := []byte{}

	log.Printf("handleStatus: %d %d", m, p)

	if join, ok := queue[m]; ok {
		if _, pok := join.Participants[p]; pok {
			log.Printf("tx from queue: %d, %s", *join.sid, join.Tx)
			if *join.sid == p && join.Tx != nil {
				res = &multijoin.JoinStatusResponse{
					Tx:    &join.Tx.Unsigned,
					Error: "",
				}
				json.NewEncoder(w).Encode(res)
				return
			}
		}
		// https://github.com/btcsuite/btcwallet/issues/619
		// this could use bitcoin rpc, but I think start with following
		// or maybe implement only what I need https://github.com/bitcoin/bips/blob/master/bip-0174.mediawiki
		//
		//   OPTION sign in sequence, the old fasion way
		//
		//   track participants individually
		//   first participant gets result of CreateTransaction
		//   they sign and post to /sig
		//   once recieved, then next request from next participant will recieve the partially signed
		//   also need a watchdog, if any participant is not heard from in max time period, abort
		//   PATH TO IMPLEMENT:
		//     ✔ update handleJoin to also provide an index to the participant
		//     ✔ use that index in the path /status/join_index/participant_index
		//     return tx in sequence
		//     ✔ add a route for submission of signed tx
		//     when received, update local tx with partial, increment participant_index of join
		//     return partial tx here when request participant_index matches local participant_index
		//
		//   OPTION bitcoin rpc
		//
		//    create psbt and send to each participant
		//      need a type for this
		//
		//      ```
		//      createpsbt [{"txid":"hex","vout":n,"sequence":n},...] [{"address":amount},{"data":"hex"},...] ( locktime replaceable )
		//      ```
		//
		//      Result:
		//		"psbt"        (string)  The resulting raw transaction (base64-encoded string)
		//
		//    watch for submissions of signatures
		//    join when all are submitted
		//      ```
		//      joinpsbts ["psbt",...]
		//      ```
		//
		//		Result:
		//		"psbt"          (string) The base64-encoded partially signed transaction
		//
		//      ```
		//     finalizepsbt "psbt" ( extract )
		//      ```
		//		Result:
		//		{
		//		"psbt" : "value",          (string) The base64-encoded partially signed transaction if not extracted
		//		"hex" : "value",           (string) The hex-encoded network transaction if extracted
		//		"complete" : true|false,   (boolean) If the transaction has a complete set of signatures
		//		]
		//		}
		//

	}

	// here we need to respond with the txid if we have it, meaning we are done
	if err != nil {
		res = &multijoin.JoinStatusResponse{
			Tx:    &b,
			Error: err.Error(),
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	if int(mid) > mixid {
		res = &multijoin.JoinStatusResponse{
			Tx:    &b,
			Error: "Invalid mix id",
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	res = &multijoin.JoinStatusResponse{
		Tx:    &b,
		Error: "",
	}
	json.NewEncoder(w).Encode(res)

}

func handleComplete(w http.ResponseWriter, req *http.Request) {
	params := strings.Split(req.URL.Path[len("/complete/"):], "/")
	id := params[0]
	participantid := params[1]
	mid, err := strconv.ParseInt(id, 10, 32)
	if err != nil {
		log.Printf("unable to parse path")
	}
	m := int(mid)
	pid, err := strconv.ParseInt(participantid, 10, 32)
	if err != nil {
		log.Printf("unable to parse path")
	}
	p := int(pid)

	queue[m].cid[p] = true
	n := 0
	for _, c := range queue[m].cid {
		if c {
			n++
		}
	}
	if n >= N_PARTICIPANTS {
		txid, err := fundr.Bitcoin.SendTx(queue[m].Tx.String())
		if err != nil {
			log.Printf("send error: %s", err.Error())
		} else {
			log.Printf("transaction sent: %s", txid)
		}

	}
}

func handleUpdate(w http.ResponseWriter, req *http.Request) {
	id := req.URL.Path[len("/update/"):]
	mid, err := strconv.ParseInt(id, 10, 32)
	if err != nil {
		log.Printf("unable to parse path")
	}
	m := int(mid)

	tx := &multijoin.TxResponse{
		Signed: &queue[m].Tx.Signed,
		TxId:   &queue[m].Tx.TxId,
	}
	res := &multijoin.JoinUpdateResponse{
		Response: tx,
		Error:    "",
	}
	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		log.Printf("encode error: %s", err.Error())
	}

}

func handleSig(w http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	b := []byte{}
	var sig multijoin.TransactionSubmission

	err := decoder.Decode(&sig)
	if err != nil {
		log.Printf("decode error: %s", err.Error())
		res := &multijoin.JoinStatusResponse{
			Tx:    &b,
			Error: err.Error(),
		}
		err = json.NewEncoder(w).Encode(res)
		if err != nil {
			log.Printf("encode error: %s", err.Error())
		}
		return
	}

	signed, err := hex.DecodeString(sig.Tx)
	if err != nil {
		log.Printf("hex decode error: %s", err.Error())
	}
	log.Printf("sig received: %x", signed)
	queue[sig.Id].Tx.Unsigned = signed
	*queue[sig.Id].sid++

	if *queue[sig.Id].sid > N_PARTICIPANTS {
		log.Printf("sending tx: %s", queue[sig.Id].Tx.TxId)
		queue[sig.Id].Tx.Signed = signed
		wtx := wire.NewMsgTx(2)
		r := bytes.NewReader(queue[sig.Id].Tx.Signed)
		wtx.Deserialize(r)
		queue[sig.Id].Tx.TxId = wtx.TxHash().String()

		// queue[sig.Id].Tx.TxId = txid
		log.Printf("tx sent: %s", queue[sig.Id].Tx.TxId)
	}

}

func main() {
	plugin = glightning.NewPlugin(onInit)
	fundr = &funder.Funder{}
	fundr.Lightning = glightning.NewLightning()
	queue = make(map[int]JoinQueue, 0)

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
	cfg, err := fundr.Lightning.ListConfigs()
	if err != nil {
		log.Fatal(err)
	}
	fundr.Bitcoin = wallet.NewBitcoinWallet(cfg)

	switch cfg["network"] {
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
	http.HandleFunc("/sig", handleSig)
	http.HandleFunc("/complete/", handleComplete)
	http.HandleFunc("/update/", handleUpdate)

	log.Printf("listening: %s", options["multi-join-port"])

	go (func() {
		log.Fatal(http.ListenAndServe(":"+options["multi-join-port"], nil))
	})()
}
