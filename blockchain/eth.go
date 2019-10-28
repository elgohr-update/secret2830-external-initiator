package blockchain

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/smartcontractkit/external-initiator/subscriber"
	"math/big"
)

const ETH = "ethereum"

func toBlockNumArg(number *big.Int) string {
	if number == nil {
		return "latest"
	}
	return hexutil.EncodeBig(number)
}

func toFilterArg(q ethereum.FilterQuery) (interface{}, error) {
	arg := map[string]interface{}{
		"address": q.Addresses,
		"topics":  q.Topics,
	}
	if q.BlockHash != nil {
		arg["blockHash"] = *q.BlockHash
		if q.FromBlock != nil || q.ToBlock != nil {
			return nil, fmt.Errorf("cannot specify both BlockHash and FromBlock/ToBlock")
		}
	} else {
		if q.FromBlock == nil {
			arg["fromBlock"] = "0x0"
		} else {
			arg["fromBlock"] = toBlockNumArg(q.FromBlock)
		}
		arg["toBlock"] = toBlockNumArg(q.ToBlock)
	}
	return arg, nil
}

type jsonrpcMessage struct {
	Version string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Error   *interface{}    `json:"error,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

type EthFilterMessage struct {
	fq ethereum.FilterQuery
}

func CreateEthFilterMessage(addressesStr []string, topicsStr []string) EthFilterMessage {
	var addresses []common.Address
	for _, a := range addressesStr {
		addresses = append(addresses, common.HexToAddress(a))
	}

	var topics [][]common.Hash
	var t []common.Hash
	for _, value := range topicsStr {
		if len(value) < 1 {
			continue
		}
		t = append(t, common.HexToHash(value))
	}
	topics = append(topics, t)

	return EthFilterMessage{
		fq: ethereum.FilterQuery{
			Addresses: addresses,
			Topics:    topics,
		},
	}
}

func (fm EthFilterMessage) Json() []byte {
	filter, err := toFilterArg(fm.fq)
	if err != nil {
		return nil
	}

	filterBytes, err := json.Marshal(filter)
	if err != nil {
		return nil
	}

	msg := jsonrpcMessage{
		Version: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "eth_subscribe",
		Params:  json.RawMessage(`["logs",` + string(filterBytes) + `]`),
	}

	bytes, err := json.Marshal(msg)
	if err != nil {
		return nil
	}

	return bytes
}

type EthParser struct{}

func (parser EthParser) ParseResponse(data []byte) ([]subscriber.Event, bool) {
	// All ETH subscription responses should be relevant,
	// so we can just push them
	return []subscriber.Event{data}, true
}
