package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ethpandaops/ethereum-address-metrics-exporter/pkg/exporter/api"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

// TransactionInputPrefix exposes metrics for transactions sent to a contract where the input data starts with a configured prefix.
type TransactionInputPrefix struct {
	client                                     api.ExecutionClient
	log                                        logrus.FieldLogger
	TransactionInputPrefixTransactions         prometheus.CounterVec
	TransactionInputPrefixMatchedTransactions  prometheus.CounterVec
	TransactionInputPrefixLastTransactionBlock prometheus.GaugeVec
	TransactionInputPrefixLastCheckedBlock     prometheus.GaugeVec
	TransactionInputPrefixError                prometheus.CounterVec
	checkInterval                              time.Duration
	BlockIncrement                             int
	stateFile                                  string
	addresses                                  []*AddressTransactionInputPrefix
	baseLabelsMap                              map[string]int
	transactionLabelsMap                       map[string]int
}

type AddressTransactionInputPrefix struct {
	LastKnownBlock int64             `yaml:"last_known_block"`
	StartBlock     *int64            `yaml:"startBlock"`
	MethodPrefix   string            `yaml:"methodPrefix"`
	InputPrefix    string            `yaml:"inputPrefix"`
	Prefix         string            `yaml:"prefix"`
	BytesArgIndex  *int              `yaml:"bytesArgIndex"`
	Contract       string            `yaml:"contract"`
	Name           string            `yaml:"name"`
	Labels         map[string]string `yaml:"labels"`
	initialized    bool
}

type transactionInputPrefixState struct {
	LastKnownBlocks map[string]int64 `json:"last_known_blocks"`
}

const (
	NameTransactionInputPrefix = "transaction_input_prefix"
)

func (n *TransactionInputPrefix) Name() string {
	return NameTransactionInputPrefix
}

// NewTransactionInputPrefix returns a new TransactionInputPrefix instance.
func NewTransactionInputPrefix(client api.ExecutionClient, log logrus.FieldLogger, checkInterval time.Duration, blockIncrement int, namespace string, stateFile string, constLabels map[string]string, addresses []*AddressTransactionInputPrefix) TransactionInputPrefix {
	namespace += "_" + NameTransactionInputPrefix

	baseLabelsMap := transactionInputPrefixLabelsMap(addresses, false)
	transactionLabelsMap := transactionInputPrefixLabelsMap(addresses, true)

	instance := TransactionInputPrefix{
		client:               client,
		log:                  log.WithField("module", NameTransactionInputPrefix),
		addresses:            addresses,
		checkInterval:        checkInterval,
		BlockIncrement:       blockIncrement,
		stateFile:            stateFile,
		baseLabelsMap:        baseLabelsMap,
		transactionLabelsMap: transactionLabelsMap,
		TransactionInputPrefixTransactions: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace:   namespace,
				Name:        "transactions_total",
				Help:        "The total transactions to a contract matching the configured method prefix, split by whether input starts with the configured input prefix.",
				ConstLabels: constLabels,
			},
			labelsFromMap(transactionLabelsMap),
		),
		TransactionInputPrefixMatchedTransactions: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace:   namespace,
				Name:        "matched_transactions_total",
				Help:        "The total transactions to a contract matching both the configured method prefix and input prefix.",
				ConstLabels: constLabels,
			},
			labelsFromMap(baseLabelsMap),
		),
		TransactionInputPrefixLastTransactionBlock: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   namespace,
				Name:        "last_transaction_block",
				Help:        "The latest block with a transaction to a contract matching the configured method prefix, split by whether input starts with the configured input prefix.",
				ConstLabels: constLabels,
			},
			labelsFromMap(transactionLabelsMap),
		),
		TransactionInputPrefixLastCheckedBlock: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   namespace,
				Name:        "last_checked_block",
				Help:        "The latest block checked for contract transactions matching the configured method prefix.",
				ConstLabels: constLabels,
			},
			labelsFromMap(baseLabelsMap),
		),
		TransactionInputPrefixError: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace:   namespace,
				Name:        "errors_total",
				Help:        "The total errors when checking contract transaction input prefixes.",
				ConstLabels: constLabels,
			},
			labelsFromMap(baseLabelsMap),
		),
	}

	prometheus.MustRegister(instance.TransactionInputPrefixTransactions)
	prometheus.MustRegister(instance.TransactionInputPrefixMatchedTransactions)
	prometheus.MustRegister(instance.TransactionInputPrefixLastTransactionBlock)
	prometheus.MustRegister(instance.TransactionInputPrefixLastCheckedBlock)
	prometheus.MustRegister(instance.TransactionInputPrefixError)

	instance.loadState()

	return instance
}

func transactionInputPrefixLabelsMap(addresses []*AddressTransactionInputPrefix, includeMatched bool) map[string]int {
	labelsMap := map[string]int{
		LabelName:          0,
		LabelContract:      1,
		LabelMethodPrefix:  2,
		LabelInputPrefix:   3,
		LabelBytesArgIndex: 4,
	}

	if includeMatched {
		labelsMap[LabelMatched] = len(labelsMap)
	}

	for address := range addresses {
		for label := range addresses[address].Labels {
			if label == LabelMatched || label == LabelBytesArgIndex {
				continue
			}

			if _, ok := labelsMap[label]; !ok {
				labelsMap[label] = len(labelsMap)
			}
		}
	}

	return labelsMap
}

func labelsFromMap(labelsMap map[string]int) []string {
	labels := make([]string, len(labelsMap))
	for label, index := range labelsMap {
		labels[index] = label
	}

	return labels
}

func (n *TransactionInputPrefix) loadState() {
	if n.stateFile == "" {
		return
	}

	data, err := os.ReadFile(n.stateFile)
	if err != nil {
		if !os.IsNotExist(err) {
			n.log.WithError(err).WithField("state_file", n.stateFile).Warn("Failed to read transaction input prefix state")
		}

		return
	}

	state := &transactionInputPrefixState{}
	if err := json.Unmarshal(data, state); err != nil {
		n.log.WithError(err).WithField("state_file", n.stateFile).Warn("Failed to parse transaction input prefix state")
		return
	}

	for _, address := range n.addresses {
		if address.LastKnownBlock > 0 {
			address.initialized = true
		}

		lastKnownBlock, ok := state.LastKnownBlocks[address.Name]
		if ok {
			if lastKnownBlock > address.LastKnownBlock {
				address.LastKnownBlock = lastKnownBlock
			}

			address.initialized = true
		}
	}
}

func (n *TransactionInputPrefix) saveState() error {
	if n.stateFile == "" {
		return nil
	}

	state := &transactionInputPrefixState{
		LastKnownBlocks: make(map[string]int64, len(n.addresses)),
	}

	for _, address := range n.addresses {
		state.LastKnownBlocks[address.Name] = address.LastKnownBlock
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(n.stateFile), 0o755); err != nil {
		return err
	}

	tmpStateFile := n.stateFile + ".tmp"
	if err := os.WriteFile(tmpStateFile, data, 0o644); err != nil {
		return err
	}

	return os.Rename(tmpStateFile, n.stateFile)
}

func (n *TransactionInputPrefix) Start(ctx context.Context) {
	n.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(n.checkInterval):
			n.tick(ctx)
		}
	}
}

//nolint:unparam // context will be used in the future
func (n *TransactionInputPrefix) tick(ctx context.Context) {
	for _, address := range n.addresses {
		err := n.getTransactions(address)

		if err != nil {
			n.log.WithError(err).WithField("address", address).Error("Failed to get transaction input prefix metrics")
		}
	}
}

func (n *TransactionInputPrefix) getBaseLabelValues(address *AddressTransactionInputPrefix, methodPrefix string, inputPrefix string) []string {
	values := make([]string, len(n.baseLabelsMap))

	for label, index := range n.baseLabelsMap {
		switch label {
		case LabelName:
			values[index] = address.Name
		case LabelContract:
			values[index] = address.Contract
		case LabelMethodPrefix:
			values[index] = methodPrefix
		case LabelInputPrefix:
			values[index] = inputPrefix
		case LabelBytesArgIndex:
			values[index] = address.bytesArgIndexLabel()
		default:
			if address.Labels != nil && address.Labels[label] != "" {
				values[index] = address.Labels[label]
			} else {
				values[index] = LabelDefaultValue
			}
		}
	}

	return values
}

func (n *TransactionInputPrefix) getTransactionLabelValues(address *AddressTransactionInputPrefix, methodPrefix string, inputPrefix string, matched bool) []string {
	values := make([]string, len(n.transactionLabelsMap))

	for label, index := range n.transactionLabelsMap {
		switch label {
		case LabelName:
			values[index] = address.Name
		case LabelContract:
			values[index] = address.Contract
		case LabelMethodPrefix:
			values[index] = methodPrefix
		case LabelInputPrefix:
			values[index] = inputPrefix
		case LabelBytesArgIndex:
			values[index] = address.bytesArgIndexLabel()
		case LabelMatched:
			values[index] = strconv.FormatBool(matched)
		default:
			if address.Labels != nil && address.Labels[label] != "" {
				values[index] = address.Labels[label]
			} else {
				values[index] = LabelDefaultValue
			}
		}
	}

	return values
}

func (a *AddressTransactionInputPrefix) effectiveInputPrefix() string {
	if a.InputPrefix != "" {
		return a.InputPrefix
	}

	return a.Prefix
}

func (a *AddressTransactionInputPrefix) effectiveMethodPrefix() string {
	if a.MethodPrefix != "" {
		return a.MethodPrefix
	}

	return a.effectiveInputPrefix()
}

func (a *AddressTransactionInputPrefix) bytesArgIndexLabel() string {
	if a.BytesArgIndex == nil {
		return LabelDefaultValue
	}

	return strconv.Itoa(*a.BytesArgIndex)
}

func (n *TransactionInputPrefix) getTransactions(address *AddressTransactionInputPrefix) (err error) {
	methodPrefix := address.effectiveMethodPrefix()
	inputPrefix := address.effectiveInputPrefix()

	defer func() {
		if err != nil {
			n.TransactionInputPrefixError.WithLabelValues(n.getBaseLabelValues(address, methodPrefix, inputPrefix)...).Inc()
		}
	}()

	if address.BytesArgIndex != nil && address.MethodPrefix == "" {
		return fmt.Errorf("methodPrefix is required when bytesArgIndex is set")
	}

	methodPrefix, err = normalizeHexPrefix(methodPrefix)
	if err != nil {
		return err
	}

	inputPrefix, err = normalizeHexPrefix(inputPrefix)
	if err != nil {
		return err
	}

	currentBlockStr, err := n.client.ETHGetBlockNumber()
	if err != nil {
		return err
	}

	currentBlock := hexStringToInt64(currentBlockStr)
	blockIncrement := n.BlockIncrement
	if blockIncrement <= 0 {
		blockIncrement = 1
	}

	nextStartBlock := address.nextStartBlock(currentBlock)

	if nextStartBlock > currentBlock {
		n.TransactionInputPrefixLastCheckedBlock.WithLabelValues(n.getBaseLabelValues(address, methodPrefix, inputPrefix)...).Set(float64(currentBlock))
		return nil
	}

	nextEndBlock := nextStartBlock + int64(blockIncrement) - 1
	if nextEndBlock > currentBlock {
		nextEndBlock = currentBlock
	}

	for blockNumber := nextStartBlock; blockNumber <= nextEndBlock; blockNumber++ {
		block, err := n.client.ETHGetBlockByNumber(int64ToHexString(blockNumber), true)
		if err != nil {
			return err
		}

		for _, transaction := range block.Transactions {
			if transaction.To == nil || !sameHexAddress(*transaction.To, address.Contract) {
				continue
			}

			if !hasHexPrefix(transaction.Input, methodPrefix) {
				continue
			}

			matched, err := address.matchesInputPrefix(transaction.Input, inputPrefix)
			if err != nil {
				return err
			}

			labelValues := n.getTransactionLabelValues(address, methodPrefix, inputPrefix, matched)

			n.TransactionInputPrefixTransactions.WithLabelValues(labelValues...).Inc()
			if matched {
				n.TransactionInputPrefixMatchedTransactions.WithLabelValues(n.getBaseLabelValues(address, methodPrefix, inputPrefix)...).Inc()
			}
			n.TransactionInputPrefixLastTransactionBlock.WithLabelValues(labelValues...).Set(float64(blockNumber))
		}
	}

	address.LastKnownBlock = nextEndBlock
	address.initialized = true
	n.TransactionInputPrefixLastCheckedBlock.WithLabelValues(n.getBaseLabelValues(address, methodPrefix, inputPrefix)...).Set(float64(nextEndBlock))

	return n.saveState()
}

func (a *AddressTransactionInputPrefix) matchesInputPrefix(calldata string, inputPrefix string) (bool, error) {
	if a.BytesArgIndex == nil {
		return hasHexPrefix(calldata, inputPrefix), nil
	}

	bytesArg, err := decodeABIBytesArgument(calldata, *a.BytesArgIndex)
	if err != nil {
		return false, err
	}

	return hasHexPrefix(bytesArg, inputPrefix), nil
}

func (a *AddressTransactionInputPrefix) nextStartBlock(currentBlock int64) int64 {
	if a.initialized || a.LastKnownBlock > 0 {
		return a.LastKnownBlock + 1
	}

	if a.StartBlock != nil {
		return *a.StartBlock
	}

	return currentBlock
}
