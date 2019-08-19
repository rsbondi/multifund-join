package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/niftynei/glightning/glightning"
	"github.com/niftynei/glightning/jrpc2"
	"github.com/rsbondi/multifund-join/multijoin"
	"github.com/rsbondi/multifund/wallet"
)

const JoinMultiDescription = `Use external wallet funding feature to build a transaction to fund multiple channels
among multiple peers{channels} is an array of object{"id" string, "satoshi" int, "announce" bool}`

type MultiChannelJoin struct {
	Host     string                        `json:"host"`
	Channels []glightning.FundChannelStart `json:"channels"`
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

func waitForStatus(id int, pid int, host string) {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ticker.C:
			url := fmt.Sprintf("%s/status/%d/%d", host, id, pid)
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
				ticker.Stop()
				return
			}
		}
	}
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
	log.Printf("pid: &d", pid)
	go waitForStatus(joinid, pid, m.Host)
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
