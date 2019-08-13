package main

import (
	"log"

	"github.com/niftynei/glightning/jrpc2"
	"github.com/rsbondi/multifund/rpc"
	"github.com/rsbondi/multifund/wallet"
)

const JoinMultiDescription = `Use external wallet funding feature to build a transaction to fund multiple channels
among multiple peers{channels} is an array of object{"id" string, "satoshi" int, "announce" bool}`

type MultiChannelJoin struct {
	Channels []rpc.FundChannelStartRequest
}

func (m *MultiChannelJoin) Call() (jrpc2.Result, error) {
	return joinMultiStart(&m.Channels)
}

func (f *MultiChannelJoin) Name() string {
	return "join_multi_start"
}

func (f *MultiChannelJoin) New() interface{} {
	return &MultiChannelJoin{}
}

func joinMultiStart(chans *[]rpc.FundChannelStartRequest) (jrpc2.Result, error) {
	info, err := fundr.GetChannelAddresses(chans)
	if err != nil {
		cancelMulti(info.Outputs)
		return nil, err
	}

	// here is where we deviate from the local version
	// before we created the transaction, now we have to send our info to the server
	// we need to be listening for the server's response
	// so we need to send and bail out
	// when we get the transaction, we need to verify all of our channel outputs and change address amounts
	// if correct, sign, if not screw you

	return nil, nil
}

func cancelMulti(outputs map[string]*wallet.Outputs) {
	for k, _ := range outputs {
		_, err := rpc.FundChannelCancel(k)
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
