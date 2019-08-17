package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/niftynei/glightning/jrpc2"
	"github.com/rsbondi/multifund-join/multijoin"
	"github.com/rsbondi/multifund/rpc"
	"github.com/rsbondi/multifund/wallet"
)

const JoinMultiDescription = `Use external wallet funding feature to build a transaction to fund multiple channels
among multiple peers{channels} is an array of object{"id" string, "satoshi" int, "announce" bool}`

type MultiChannelJoin struct {
	Host     string                        `json:"host"`
	Channels []rpc.FundChannelStartRequest `json:"channels"`
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

func joinMultiStart(m *MultiChannelJoin) (jrpc2.Result, error) {
	info, err := fundr.GetChannelAddresses(&m.Channels)
	if err != nil {
		cancelMulti(m.Channels)
		return nil, err
	}

	jsoncall, err := json.Marshal(&info)
	if err != nil {
		log.Printf("unable to marshall json: %s", err.Error())
		return nil, err
	}
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/join", m.Host), bytes.NewBuffer(jsoncall))
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
	return result.Response, nil
}

func cancelMulti(chans []rpc.FundChannelStartRequest) {
	for _, ch := range chans {
		_, err := rpc.FundChannelCancel(ch.Id)
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
