package nearapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

type StatusResult struct {
	Status struct {
		Version struct {
			Version string `json:"version"`
			Build   string `json:"build"`
		} `json:"version"`
		ChainId string `json:"chain_id"`
		RpcAddr string `json:"rpc_addr"`
		//Validators []string `json:"validators"`
		SyncInfo struct {
			LatestBlockHash   string `json:"latest_block_hash"`
			LatestBlockHeight uint64 `json:"latest_block_height"`
			LatestStateRoot   string `json:"latest_state_root"`
			LatestBlockTime   string `json:"latest_block_time"`
			Syncing           bool   `json:"syncing"`
		} `json:"sync_info"`
	} `json:"result_status"`
}

type Validator struct {
	AccountId string `json:"account_id"`
	PublicKey string `json:"public_key"`
	Stake     string `json:"stake"`
}

type ValidatorsResult struct {
	Validators struct {
		CurrentValidators []struct {
			Validator
			IsSlashed         bool  `json:"is_slashed"`
			Shards            []int `json:"shards"`
			NumProducedBlocks int64 `json:"num_produced_blocks"`
			NumExpectedBlocks int64 `json:"num_expected_blocks"`
		} `json:"current_validators"`
		NextValidators []struct {
			Validator
			Shards []int `json:"shards"`
		} `json:"next_validators"`
		CurrentProposals []struct {
			Validator
		} `json:"current_proposals"`
		EpochStartHeight int64 `json:"epoch_start_height"`
		PrevEpochKickOut []struct {
			AccountId string                            `json:"account_id"`
			Reason    map[string]map[string]interface{} `json:"reason"`
		} `json:"prev_epoch_kickout"`
	} `json:"result_validators"`
}

type Result struct {
	StatusResult
	ValidatorsResult
}

type Client struct {
	httpClient *http.Client
	Endpoint   string
	ctx        context.Context
}

func NewClientWithContext(ctx context.Context, endpoint string) *Client {
	var netTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 12 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 12 * time.Second,
	}

	timeout := time.Duration(12 * time.Second)
	httpClient := &http.Client{
		Timeout:   timeout,
		Transport: netTransport,
	}
	return &Client{
		Endpoint:   endpoint,
		httpClient: httpClient,
		ctx:        ctx,
	}
}

func (c *Client) do(method string, params interface{}) (string, error) {
	payload, err := json.Marshal(map[string]string{
		"query": method,
	})

	if params != "" {
		type Payload struct {
			JsonRPC string      `json:"jsonrpc"`
			Id      string      `json:"id"`
			Method  string      `json:"method"`
			Params  interface{} `json:"params"`
		}
		p := Payload{
			JsonRPC: "2.0",
			Id:      "dontcare",
			Method:  method,
			Params:  params,
		}
		payload, err = json.Marshal(p)
		if err != nil {
			log.Println(err)
		}
	}
	ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequest("POST", c.Endpoint, bytes.NewBuffer(payload))
	req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("User-Agent", "Go-Warchest Bot")
	if err != nil {
		log.Fatalln(err)
	}

	r, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("Failed to connect to %s\n", c.Endpoint)
		return "", nil
	}
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Failed to read body: %s\n", c.Endpoint)
		return "", nil
	}

	//log.Printf("body %s\n %s\n", method, body)

	return string(body), nil
}

func (c *Client) Get(method string, variables interface{}) (*Result, error) {
	res, err := c.do(method, variables)
	if err != nil {
		return nil, err
	}
	var d Result

	//log.Printf("res1 %s\n %s\n", method, res)

	res = strings.Replace(res, "result", fmt.Sprintf("%s_%s", "result", method), -1)
	r := bytes.NewReader([]byte(res))
	err2 := json.NewDecoder(r).Decode(&d)
	if err2 != nil {
		log.Println(err2)
		return nil, err2
	}
	//log.Printf("res2 %s\n %s\n", method, res)

	return &d, nil
}
