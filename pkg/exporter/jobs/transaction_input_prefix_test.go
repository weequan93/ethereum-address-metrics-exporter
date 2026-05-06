package jobs

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ethpandaops/ethereum-address-metrics-exporter/pkg/exporter/api"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
)

type transactionInputPrefixMockClient struct {
	currentBlock string
	blocks       map[string]*api.ETHBlock
}

func (m *transactionInputPrefixMockClient) ETHCall(transaction *api.ETHCallTransaction, block string) (string, error) {
	return "", nil
}

func (m *transactionInputPrefixMockClient) ETHGetBalance(address string, block string) (string, error) {
	return "", nil
}

func (m *transactionInputPrefixMockClient) ETHGetBlockByNumber(block string, fullTransactions bool) (*api.ETHBlock, error) {
	if !fullTransactions {
		return nil, fmt.Errorf("expected full transactions")
	}

	ethBlock, ok := m.blocks[block]
	if !ok {
		return nil, fmt.Errorf("missing block %s", block)
	}

	return ethBlock, nil
}

func (m *transactionInputPrefixMockClient) ETHGetEvent(address string, topic string, fromBlock string, toBlock string) ([]api.ETHLogEntry, error) {
	return nil, nil
}

func (m *transactionInputPrefixMockClient) ETHGetBlockNumber() (string, error) {
	return m.currentBlock, nil
}

func TestTransactionInputPrefixCountsMatchedAndUnmatchedTransactions(t *testing.T) {
	contract := "0x00000000000000000000000000000000000000aa"
	otherContract := "0x00000000000000000000000000000000000000bb"
	startBlock := int64(1)

	client := &transactionInputPrefixMockClient{
		currentBlock: "0x2",
		blocks: map[string]*api.ETHBlock{
			"0x1": {
				Number: "0x1",
				Transactions: []api.ETHTransaction{
					{
						To:    &contract,
						Input: "0xaaaaaaaa30ff",
					},
					{
						To:    &otherContract,
						Input: "0xaaaaaaaa30ff",
					},
				},
			},
			"0x2": {
				Number: "0x2",
				Transactions: []api.ETHTransaction{
					{
						To:    &contract,
						Input: "0xaaaaaaaa40ff",
					},
					{
						To:    &contract,
						Input: "0xbbbbbbbb30ff",
					},
				},
			},
		},
	}

	address := &AddressTransactionInputPrefix{
		Name:         "batch-poster",
		Contract:     contract,
		StartBlock:   &startBlock,
		MethodPrefix: "0xaaaaaaaa",
		InputPrefix:  "0xaaaaaaaa30",
		Labels: map[string]string{
			"source": "dac",
		},
	}

	job := NewTransactionInputPrefix(
		client,
		logrus.New(),
		time.Hour,
		2,
		"test_transaction_input_prefix",
		"",
		nil,
		[]*AddressTransactionInputPrefix{address},
	)

	job.tick(context.Background())

	trueLabels := job.getTransactionLabelValues(address, "0xaaaaaaaa", "0xaaaaaaaa30", true)
	falseLabels := job.getTransactionLabelValues(address, "0xaaaaaaaa", "0xaaaaaaaa30", false)
	baseLabels := job.getBaseLabelValues(address, "0xaaaaaaaa", "0xaaaaaaaa30")

	if got := testutil.ToFloat64(job.TransactionInputPrefixTransactions.WithLabelValues(trueLabels...)); got != 1 {
		t.Fatalf("matched transactions = %v, want 1", got)
	}

	if got := testutil.ToFloat64(job.TransactionInputPrefixTransactions.WithLabelValues(falseLabels...)); got != 1 {
		t.Fatalf("unmatched transactions = %v, want 1", got)
	}

	if got := testutil.ToFloat64(job.TransactionInputPrefixMatchedTransactions.WithLabelValues(baseLabels...)); got != 1 {
		t.Fatalf("matched-only transactions = %v, want 1", got)
	}

	if got := testutil.ToFloat64(job.TransactionInputPrefixLastCheckedBlock.WithLabelValues(baseLabels...)); got != 2 {
		t.Fatalf("last checked block = %v, want 2", got)
	}

	if address.LastKnownBlock != 2 {
		t.Fatalf("last known block = %v, want 2", address.LastKnownBlock)
	}

	job.tick(context.Background())

	if got := testutil.ToFloat64(job.TransactionInputPrefixTransactions.WithLabelValues(trueLabels...)); got != 1 {
		t.Fatalf("matched transactions after second tick = %v, want 1", got)
	}

	if got := testutil.ToFloat64(job.TransactionInputPrefixTransactions.WithLabelValues(falseLabels...)); got != 1 {
		t.Fatalf("unmatched transactions after second tick = %v, want 1", got)
	}

	if got := testutil.ToFloat64(job.TransactionInputPrefixMatchedTransactions.WithLabelValues(baseLabels...)); got != 1 {
		t.Fatalf("matched-only transactions after second tick = %v, want 1", got)
	}
}

func TestTransactionInputPrefixMatchesDecodedBytesArgument(t *testing.T) {
	contract := "0x00000000000000000000000000000000000000aa"
	bytesArgIndex := 1

	client := &transactionInputPrefixMockClient{
		currentBlock: "0x1",
		blocks: map[string]*api.ETHBlock{
			"0x1": {
				Number: "0x1",
				Transactions: []api.ETHTransaction{
					{
						To:    &contract,
						Input: abiCalldataWithSecondBytesArg("0xaaaaaaaa", "30ff00"),
					},
					{
						To:    &contract,
						Input: abiCalldataWithSecondBytesArg("0xaaaaaaaa", "40ff00"),
					},
				},
			},
		},
	}

	address := &AddressTransactionInputPrefix{
		Name:          "batch-poster",
		Contract:      contract,
		MethodPrefix:  "0xaaaaaaaa",
		InputPrefix:   "0x30",
		BytesArgIndex: &bytesArgIndex,
	}

	job := NewTransactionInputPrefix(
		client,
		logrus.New(),
		time.Hour,
		1,
		"test_transaction_input_prefix_bytes_arg",
		"",
		nil,
		[]*AddressTransactionInputPrefix{address},
	)

	job.tick(context.Background())

	trueLabels := job.getTransactionLabelValues(address, "0xaaaaaaaa", "0x30", true)
	falseLabels := job.getTransactionLabelValues(address, "0xaaaaaaaa", "0x30", false)

	if got := testutil.ToFloat64(job.TransactionInputPrefixTransactions.WithLabelValues(trueLabels...)); got != 1 {
		t.Fatalf("matched decoded bytes transactions = %v, want 1", got)
	}

	if got := testutil.ToFloat64(job.TransactionInputPrefixTransactions.WithLabelValues(falseLabels...)); got != 1 {
		t.Fatalf("unmatched decoded bytes transactions = %v, want 1", got)
	}

	baseLabels := job.getBaseLabelValues(address, "0xaaaaaaaa", "0x30")
	if got := testutil.ToFloat64(job.TransactionInputPrefixMatchedTransactions.WithLabelValues(baseLabels...)); got != 1 {
		t.Fatalf("matched-only decoded bytes transactions = %v, want 1", got)
	}
}

func TestTransactionInputPrefixPersistsLastKnownBlock(t *testing.T) {
	contract := "0x00000000000000000000000000000000000000aa"
	stateFile := t.TempDir() + "/transaction-input-prefix-state.json"

	client := &transactionInputPrefixMockClient{
		currentBlock: "0x1",
		blocks: map[string]*api.ETHBlock{
			"0x1": {
				Number: "0x1",
				Transactions: []api.ETHTransaction{
					{
						To:    &contract,
						Input: "0xaaaaaaaa30ff",
					},
				},
			},
		},
	}

	address := &AddressTransactionInputPrefix{
		Name:         "batch-poster",
		Contract:     contract,
		MethodPrefix: "0xaaaaaaaa",
		InputPrefix:  "0xaaaaaaaa30",
	}

	job := NewTransactionInputPrefix(
		client,
		logrus.New(),
		time.Hour,
		1,
		"test_transaction_input_prefix_state_write",
		stateFile,
		nil,
		[]*AddressTransactionInputPrefix{address},
	)

	job.tick(context.Background())

	restartedAddress := &AddressTransactionInputPrefix{
		Name:         "batch-poster",
		Contract:     contract,
		MethodPrefix: "0xaaaaaaaa",
		InputPrefix:  "0xaaaaaaaa30",
	}

	NewTransactionInputPrefix(
		client,
		logrus.New(),
		time.Hour,
		1,
		"test_transaction_input_prefix_state_read",
		stateFile,
		nil,
		[]*AddressTransactionInputPrefix{restartedAddress},
	)

	if restartedAddress.LastKnownBlock != 1 {
		t.Fatalf("loaded last known block = %v, want 1", restartedAddress.LastKnownBlock)
	}
}

func TestTransactionInputPrefixStartsAtLatestBlockByDefault(t *testing.T) {
	contract := "0x00000000000000000000000000000000000000aa"

	client := &transactionInputPrefixMockClient{
		currentBlock: "0x2",
		blocks: map[string]*api.ETHBlock{
			"0x2": {
				Number: "0x2",
				Transactions: []api.ETHTransaction{
					{
						To:    &contract,
						Input: "0xaaaaaaaa30ff",
					},
				},
			},
		},
	}

	address := &AddressTransactionInputPrefix{
		Name:         "batch-poster",
		Contract:     contract,
		MethodPrefix: "0xaaaaaaaa",
		InputPrefix:  "0xaaaaaaaa30",
	}

	job := NewTransactionInputPrefix(
		client,
		logrus.New(),
		time.Hour,
		200,
		"test_transaction_input_prefix_latest_start",
		"",
		nil,
		[]*AddressTransactionInputPrefix{address},
	)

	job.tick(context.Background())

	if address.LastKnownBlock != 2 {
		t.Fatalf("last known block = %v, want 2", address.LastKnownBlock)
	}
}

func abiCalldataWithSecondBytesArg(methodSelector string, bytesArg string) string {
	methodSelector = strings.TrimPrefix(methodSelector, "0x")
	bytesArg = strings.TrimPrefix(bytesArg, "0x")
	paddingLength := 64 - len(bytesArg)%64
	if paddingLength == 64 {
		paddingLength = 0
	}

	return "0x" +
		methodSelector +
		abiWord("1") +
		abiWord("40") +
		abiWord(fmt.Sprintf("%x", len(bytesArg)/2)) +
		bytesArg +
		strings.Repeat("0", paddingLength)
}

func abiWord(value string) string {
	return strings.Repeat("0", 64-len(value)) + value
}
