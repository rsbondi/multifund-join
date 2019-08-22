package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/niftynei/glightning/glightning"
	"github.com/niftynei/glightning/jrpc2"
	"github.com/rsbondi/multifund-join/multijoin"
	"github.com/rsbondi/multifund/funder"
	"github.com/rsbondi/multifund/wallet"
)

const JoinMultiDescription = `Use external wallet funding feature to build a transaction to fund multiple channels
among multiple peers{channels} is an array of object{"id" string, "satoshi" int, "announce" bool}`

type MultiChannelJoin struct {
	Host     string                        `json:"host"`
	Channels []glightning.FundChannelStart `json:"channels"`
	id       int
	pid      int
	info     funder.FundingInfo
}

var joinid int

func (m *MultiChannelJoin) Call() (jrpc2.Result, error) {
	return joinMultiStart(m)
}

func (f *MultiChannelJoin) Name() string {
	return "joinmulti_start"
}

func (f *MultiChannelJoin) New() interface{} {
	return &MultiChannelJoin{}
}

func waitForCompleteTx(m *MultiChannelJoin) {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ticker.C:
			url := fmt.Sprintf("%s/update/%d", m.Host, m.id)
			log.Printf("checking update: %s", url)
			req, _ := http.NewRequest("POST", url, nil)
			client := &http.Client{Timeout: time.Second * 10}
			res, err := client.Do(req)
			if err != nil {
				log.Printf("unable to do complete: %s", err.Error())
				return
			}
			var result multijoin.JoinUpdateResponse
			err = json.NewDecoder(res.Body).Decode(&result)
			if err != nil {
				log.Printf("unable to decode update: %s", err.Error())
				return
			}
			defer res.Body.Close()

			if result.Error != "" {
				log.Printf("udate check error: %s", result.Error)
			}

			if result.Response != nil && *result.Response.TxId != "" {
				// we should be done
				tx := &wallet.Transaction{
					Signed: *result.Response.Signed,
					TxId:   *result.Response.TxId,
				}
				log.Printf("ready to complete channels")
				complete(m, *tx)
				ticker.Stop()
				return
			}
		}
	}

}

func complete(m *MultiChannelJoin, tx wallet.Transaction) {
	channels, err := fundr.CompleteChannels(tx, m.info.Outputs)
	if err != nil {
		log.Printf("unable to complete channels: %s", err.Error())
		return
	}
	log.Printf("channels complete: %v", channels)
	url := fmt.Sprintf("%s/complete/%d/%d", m.Host, m.id, m.pid)
	log.Printf("completing channels: %s", url)
	req, _ := http.NewRequest("POST", url, nil)
	client := &http.Client{Timeout: time.Second * 10}
	res, err := client.Do(req)
	if err != nil {
		log.Printf("unable to do complete: %s", err.Error())
		return
	}
	defer res.Body.Close()

}
func waitToSign(m *MultiChannelJoin) {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ticker.C:
			url := fmt.Sprintf("%s/status/%d/%d", m.Host, m.id, m.pid)
			log.Printf("checking status: %s", url)
			req, _ := http.NewRequest("POST", url, nil)
			client := &http.Client{Timeout: time.Second * 10}
			res, err := client.Do(req)
			if err != nil {
				log.Printf("unable to do request: %s", err.Error())
				return
			}
			defer res.Body.Close()

			result := &multijoin.JoinStatusResponse{}
			err = json.NewDecoder(res.Body).Decode(&result)
			if err != nil {
				log.Printf("unable to decode request: %s", err.Error())
				return
			}
			if result.Tx != nil && len(*result.Tx) > 0 {
				log.Printf("status check: %x", *result.Tx)
				// TODO spin of go routine
				tx := wallet.Transaction{
					Unsigned: *result.Tx,
				}

				err = verifyTx(tx, m)
				if err != nil {
					log.Printf("output mismatch in transaction: %s", err.Error())
					return // do not sign
				}

				fundr.Wally.Sign(&tx, m.info.Utxos)
				log.Printf("signed: %x", tx.Signed)
				sign := fmt.Sprintf("%s/sig", m.Host)
				submission := &multijoin.TransactionSubmission{
					Tx:  tx.String(),
					Id:  m.id,
					Pid: m.pid,
				}
				jsoncall, err := json.Marshal(submission)
				if err != nil {
					log.Printf("unable to marshall json: %s", err.Error())
				}
				log.Printf("json sig submission: %s", string(jsoncall))

				req, _ := http.NewRequest("POST", sign, bytes.NewBuffer(jsoncall))
				client := &http.Client{Timeout: time.Second * 10}
				res, err := client.Do(req)
				if err != nil {
					log.Printf("unable to do request: %s", err.Error())
					return
				}
				defer res.Body.Close()
				go waitForCompleteTx(m)
				ticker.Stop()
				return
			}
		}
	}
}

func verifyTx(tx wallet.Transaction, m *MultiChannelJoin) error {
	wtx := wire.NewMsgTx(2)
	r := bytes.NewReader(tx.Unsigned)
	wtx.Deserialize(r)

	// check that the tx is sending where I specified
	for _, o := range m.info.Recipients {
		log.Printf("checking recipient %s", o.Address)
		vout := -1
		for v, txout := range wtx.TxOut {
			log.Printf("checking output %x", txout.PkScript)
			addr, err := btcutil.DecodeAddress(o.Address, fundr.BitcoinNet)
			log.Printf("finding output for address: %s %d", o.Address, o.Amount)

			if err != nil {
				return err
			}

			log.Printf("finding output index: %s %x", txout.PkScript, addr.ScriptAddress())
			if hex.EncodeToString(txout.PkScript[2:]) == hex.EncodeToString(addr.ScriptAddress()) {
				if o.Amount != txout.Value {
					return errors.New("Can not find output in transaction")
				}
				vout = v
				break
			}
		}
		if vout == -1 {
			return errors.New("Can not find output in transaction")
		}
	}
	return nil
}

func joinMultiStart(m *MultiChannelJoin) (jrpc2.Result, error) {
	info, err := fundr.GetChannelAddresses(&m.Channels)
	if err != nil {
		cancelMulti(&m.Channels)
		return nil, err
	}

	jsoncall, err := json.Marshal(&info)
	if err != nil {
		log.Printf("unable to marshall json: %s", err.Error())
		return nil, err
	}
	host := fmt.Sprintf("%s/join", m.Host)
	req, _ := http.NewRequest("POST", host, bytes.NewBuffer(jsoncall))
	client := &http.Client{Timeout: time.Second * 10}
	res, err := client.Do(req)
	if err != nil {
		log.Printf("unable to do request: %s", err.Error())
		return nil, err
	}
	defer res.Body.Close()

	result := &multijoin.JoinStartResponse{}
	err = json.NewDecoder(res.Body).Decode(&result)

	if err != nil {
		log.Printf("unable to read response: %s", err.Error())
		return nil, err
	}
	if result.Error != "" {
		log.Printf("failed to queue channel open: %s", result.Error)
		return nil, errors.New(result.Error)
	}

	joinid = result.Response.Id
	pid := result.Response.Pid
	log.Printf("pid: %d", pid)
	m.id = joinid
	m.pid = pid
	m.info = *info
	go waitToSign(m)
	return result.Response, nil
}

func cancelMulti(chans *[]glightning.FundChannelStart) {
	for _, ch := range *chans {
		_, err := fundr.Lightning.CancelFundChannel(ch.Id)
		if err != nil {
			log.Printf("fundchannel_cancel error: %s", err.Error())
		}
	}
}

func closeMulti(outputs map[string]*wallet.Outputs) {
	for k, _ := range outputs {
		_, err := fundr.Lightning.CloseNormal(k)
		if err != nil {
			log.Printf("channel close error: %s", err.Error())
		}
	}
}
