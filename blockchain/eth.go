package blockchain

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/smartcontractkit/external-initiator/store"
	"github.com/smartcontractkit/external-initiator/subscriber"
	"math/big"
	"strconv"
)

const ETH = "ethereum"

type EthManager struct {
	fq filterQuery
	p  subscriber.Type
}

func CreateEthManager(p subscriber.Type, config store.EthSubscription) EthManager {
	var addresses []common.Address
	for _, a := range config.Addresses {
		addresses = append(addresses, common.HexToAddress(a))
	}

	var topics [][]common.Hash
	var t []common.Hash
	for _, value := range config.Topics {
		if len(value) < 1 {
			continue
		}
		t = append(t, common.HexToHash(value))
	}
	topics = append(topics, t)

	return EthManager{
		fq: filterQuery{
			Addresses: addresses,
			Topics:    topics,
		},
		p: p,
	}
}

func (e EthManager) GetTriggerJson() []byte {
	if e.p == subscriber.RPC && e.fq.FromBlock == "" {
		e.fq.FromBlock = "latest"
	}

	filter, err := e.fq.toMapInterface()
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
	}

	switch e.p {
	case subscriber.WS:
		msg.Method = "eth_subscribe"
		msg.Params = json.RawMessage(`["logs",` + string(filterBytes) + `]`)
	case subscriber.RPC:
		msg.Method = "eth_getLogs"
		msg.Params = json.RawMessage(`[` + string(filterBytes) + `]`)
	}

	bytes, err := json.Marshal(msg)
	if err != nil {
		return nil
	}

	return bytes
}

type ethLogResponse struct {
	LogIndex         string   `json:"logIndex"`
	BlockNumber      string   `json:"blockNumber"`
	BlockHash        string   `json:"blockHash"`
	TransactionHash  string   `json:"transactionHash"`
	TransactionIndex string   `json:"transactionIndex"`
	Address          string   `json:"address"`
	Data             string   `json:"data"`
	Topics           []string `json:"topics"`
}

func (e EthManager) ParseResponse(data []byte) ([]subscriber.Event, bool) {
	var msg jsonrpcMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, false
	}

	var rawEvents []ethLogResponse
	if err := json.Unmarshal(msg.Result, &rawEvents); err != nil {
		return nil, false
	}

	var events []subscriber.Event
	for _, evt := range rawEvents {
		if e.p == subscriber.RPC {
			// Check if we can update the "fromBlock" in the query,
			// so we only get new events from blocks we haven't queried yet
			curBlkn, err := strconv.ParseInt(evt.BlockNumber, 0, 64)
			if err != nil {
				continue
			}
			// Increment the block number by 1, since we want events from *after* this block number
			curBlkn += 1

			fromBlkn, err := strconv.ParseInt(e.fq.FromBlock, 0, 64)
			if err != nil {
				continue
			}

			// If our query "fromBlock" is "latest", or our current "fromBlock" is in the past compared to
			// the last event we received, we want to update the query
			if e.fq.FromBlock == "latest" || e.fq.FromBlock == "" || new(big.Int).SetInt64(curBlkn).Cmp(new(big.Int).SetInt64(fromBlkn)) > 0 {
				e.fq.FromBlock = evt.BlockNumber
			}
		}
		event, err := json.Marshal(evt)
		if err != nil {
			continue
		}
		events = append(events, event)
	}

	return events, true
}

type filterQuery struct {
	BlockHash *common.Hash     // used by eth_getLogs, return logs only from block with this hash
	FromBlock string           // beginning of the queried range, nil means genesis block
	ToBlock   string           // end of the range, nil means latest block
	Addresses []common.Address // restricts matches to events created by specific contracts

	// The Topic list restricts matches to particular event topics. Each event has a list
	// of topics. Topics matches a prefix of that list. An empty element slice matches any
	// topic. Non-empty elements represent an alternative that matches any of the
	// contained topics.
	//
	// Examples:
	// {} or nil          matches any topic list
	// {{A}}              matches topic A in first position
	// {{}, {B}}          matches any topic in first position AND B in second position
	// {{A}, {B}}         matches topic A in first position AND B in second position
	// {{A, B}, {C, D}}   matches topic (A OR B) in first position AND (C OR D) in second position
	Topics [][]common.Hash
}

func (q filterQuery) toMapInterface() (interface{}, error) {
	arg := map[string]interface{}{
		"address": q.Addresses,
		"topics":  q.Topics,
	}
	if q.BlockHash != nil {
		arg["blockHash"] = *q.BlockHash
		if q.FromBlock != "" || q.ToBlock != "" {
			return nil, fmt.Errorf("cannot specify both BlockHash and FromBlock/ToBlock")
		}
	} else {
		if q.FromBlock == "" {
			arg["fromBlock"] = "0x0"
		} else {
			arg["fromBlock"] = q.FromBlock
		}
		arg["toBlock"] = q.ToBlock
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
