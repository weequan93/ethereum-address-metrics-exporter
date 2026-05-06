package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// ExecutionClient is an interface for executing RPC calls to the Ethereum node.
type ExecutionClient interface {
	// ETHCall executes a new message call immediately without creating a transaction on the block chain.
	ETHCall(transaction *ETHCallTransaction, block string) (string, error)
	// ETHGetBalance returns the balance of the account of given address.
	ETHGetBalance(address string, block string) (string, error)
	// ETHGetBlockByNumber returns block details by block number.
	ETHGetBlockByNumber(block string, fullTransactions bool) (*ETHBlock, error)
	// ETHGetEvent returns the event logs for a given address and topic.
	ETHGetEvent(address string, topic string, fromBlock string, toBlock string) ([]ETHLogEntry, error)
	// ETHGetBlockNumber returns the latest block number.
	ETHGetBlockNumber() (string, error)
}

type ETHBlock struct {
	Number       string           `json:"number"`
	Hash         string           `json:"hash"`
	ParentHash   string           `json:"parentHash"`
	Timestamp    string           `json:"timestamp"`
	Transactions []ETHTransaction `json:"transactions"`
}

type ETHTransaction struct {
	Hash        string  `json:"hash"`
	From        string  `json:"from"`
	To          *string `json:"to"`
	Input       string  `json:"input"`
	BlockHash   string  `json:"blockHash"`
	BlockNumber string  `json:"blockNumber"`
}

type ETHLogEntry struct {
	Address          string   `json:"address"`
	Topics           []string `json:"topics"`
	Data             string   `json:"data"`
	BlockNumber      string   `json:"blockNumber"`
	TransactionHash  string   `json:"transactionHash"`
	TransactionIndex string   `json:"transactionIndex"`
	BlockHash        string   `json:"blockHash"`
	LogIndex         string   `json:"logIndex"`
	Removed          bool     `json:"removed"`
}

type ETHCallTransaction struct {
	From     *string `json:"from"`
	To       string  `json:"to"`
	Gas      *string `json:"gas"`
	GasPrice *string `json:"gasPrice"`
	Value    *string `json:"value"`
	Data     *string `json:"data"`
}

type ETHLogsFilter struct {
	FromBlock *string   `json:"fromBlock,omitempty"`
	ToBlock   *string   `json:"toBlock,omitempty"`
	Address   *string   `json:"address,omitempty"`
	Topics    *[]string `json:"topics,omitempty"`
}

type executionClient struct {
	url     string
	log     logrus.FieldLogger
	client  http.Client
	headers map[string]string

	metrics Metrics
}

// NewExecutionClient creates a new ExecutionClient.
func NewExecutionClient(log logrus.FieldLogger, namespace, url string, headers map[string]string, timeout time.Duration) ExecutionClient {
	client := http.Client{
		Timeout: timeout,
	}

	return &executionClient{
		url:     url,
		log:     log,
		client:  client,
		headers: headers,

		metrics: NewMetrics(fmt.Sprintf("%s_%s", namespace, "http")),
	}
}

type apiResponse struct {
	JSONRpc string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result"`
}

//nolint:unparam // ctx will probably be used in the future
func (e *executionClient) post(method string, params interface{}, id int) (json.RawMessage, error) {
	start := time.Now()

	httpMethod := "POST"

	e.metrics.ObserveRequest(httpMethod, e.url, method)

	var rsp *http.Response

	var err error

	defer func() {
		rspCode := "none"
		if rsp != nil {
			rspCode = fmt.Sprintf("%d", rsp.StatusCode)
		}

		e.metrics.ObserveResponse(httpMethod, e.url, method, rspCode, time.Since(start))
	}()

	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"id":      id,
		"params":  params,
	}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(httpMethod, e.url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	for k, v := range e.headers {
		req.Header.Set(k, v)
	}

	req.Header.Set("Content-Type", "application/json")

	rsp, err = e.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer rsp.Body.Close()

	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: %d", rsp.StatusCode)
	}

	data, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	resp := new(apiResponse)
	if err := json.Unmarshal(data, resp); err != nil {
		return nil, err
	}

	return resp.Result, nil
}

func (e *executionClient) ETHCall(transaction *ETHCallTransaction, block string) (string, error) {
	params := []interface{}{
		transaction,
		block,
	}

	rsp, err := e.post("eth_call", params, 1)
	if err != nil {
		return "", err
	}

	ethCall := ""
	if err := json.Unmarshal(rsp, &ethCall); err != nil {
		return "", err
	}

	return ethCall, nil
}

func (e *executionClient) ETHGetBalance(address, block string) (string, error) {
	params := []interface{}{
		address,
		block,
	}

	rsp, err := e.post("eth_getBalance", params, 1)
	if err != nil {
		return "", err
	}

	ethGetBalance := ""
	if err := json.Unmarshal(rsp, &ethGetBalance); err != nil {
		return "", err
	}

	return ethGetBalance, nil
}

func (e *executionClient) ETHGetBlockByNumber(block string, fullTransactions bool) (*ETHBlock, error) {
	params := []interface{}{
		block,
		fullTransactions,
	}

	rsp, err := e.post("eth_getBlockByNumber", params, 1)
	if err != nil {
		return nil, err
	}

	if string(rsp) == "null" {
		return nil, fmt.Errorf("block not found: %s", block)
	}

	ethBlock := &ETHBlock{}
	if err := json.Unmarshal(rsp, ethBlock); err != nil {
		return nil, err
	}

	return ethBlock, nil
}

func (e *executionClient) ETHGetEvent(address string, topic string, fromBlock string, toBlock string) ([]ETHLogEntry, error) {
	params := []ETHLogsFilter{
		{
			FromBlock: &fromBlock,
			ToBlock:   &toBlock,
			Address:   &address,
			Topics:    &[]string{topic},
		},
	}

	rsp, err := e.post("eth_getLogs", params, 1)
	if err != nil {
		return nil, err
	}

	ethGetEvent := []ETHLogEntry{}
	if err := json.Unmarshal(rsp, &ethGetEvent); err != nil {
		return nil, err
	}

	return ethGetEvent, nil
}

func (e *executionClient) ETHGetBlockNumber() (string, error) {
	params := []interface{}{}

	rsp, err := e.post("eth_blockNumber", params, 1)
	if err != nil {
		return "", err
	}

	ethCall := ""
	if err := json.Unmarshal(rsp, &ethCall); err != nil {
		return "", err
	}

	return ethCall, nil
}
