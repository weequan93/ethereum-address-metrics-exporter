package jobs

import (
	"context"
	"time"

	"github.com/ethpandaops/ethereum-address-metrics-exporter/pkg/exporter/api"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

// Event exposes metrics for smartcontract event by address, event name
type Event struct {
	client         api.ExecutionClient
	log            logrus.FieldLogger
	EventBlock     prometheus.GaugeVec
	EventError     prometheus.CounterVec
	checkInterval  time.Duration
	BlockIncrement int
	addresses      []*AddressEvent
	labelsMap      map[string]int
}

type AddressEvent struct {
	LastKnownBlock int64             `yaml:"last_known_block"`
	Topic          string            `yaml:"topic"`
	Contract       string            `yaml:"contract"`
	Name           string            `yaml:"name"`
	Labels         map[string]string `yaml:"labels"`
}

const (
	NameEvent = "event"
)

func (n *Event) Name() string {
	return NameEvent
}

// NewEvent returns a new Event instance.
func NewEvent(client api.ExecutionClient, log logrus.FieldLogger, checkInterval time.Duration, blockIncrement int, namespace string, constLabels map[string]string, addresses []*AddressEvent) Event {
	namespace += "_" + NameEvent

	labelsMap := map[string]int{
		LabelName:     0,
		LabelTopic:    1,
		LabelContract: 2,
	}

	for address := range addresses {
		for label := range addresses[address].Labels {
			if _, ok := labelsMap[label]; !ok {
				labelsMap[label] = len(labelsMap)
			}
		}
	}

	labels := make([]string, len(labelsMap))
	for label, index := range labelsMap {
		labels[index] = label
	}

	instance := Event{
		client:         client,
		log:            log.WithField("module", NameEvent),
		addresses:      addresses,
		checkInterval:  checkInterval,
		BlockIncrement: blockIncrement,
		labelsMap:      labelsMap,
		EventBlock: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   namespace,
				Name:        "event_time",
				Help:        "The time of a ethereum event by address.",
				ConstLabels: constLabels,
			},
			labels,
		),
		EventError: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace:   namespace,
				Name:        "errors_total",
				Help:        "The total errors when getting the balance of a ethereum ERC20 contract by address.",
				ConstLabels: constLabels,
			},
			labels,
		),
	}

	prometheus.MustRegister(instance.EventBlock)
	prometheus.MustRegister(instance.EventError)

	return instance
}

func (n *Event) Start(ctx context.Context) {
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
func (n *Event) tick(ctx context.Context) {
	for _, address := range n.addresses {
		err := n.getEvent(address)

		if err != nil {
			n.log.WithError(err).WithField("address", address).Error("Failed to get event address")
		}
	}
}

func (n *Event) getLabelValues(address *AddressEvent) []string {
	values := make([]string, len(n.labelsMap))

	for label, index := range n.labelsMap {
		if address.Labels != nil && address.Labels[label] != "" {
			values[index] = address.Labels[label]
		} else {
			switch label {
			case LabelName:
				values[index] = address.Name
			case LabelTopic:
				values[index] = address.Topic
			case LabelContract:
				values[index] = address.Contract
			default:
				values[index] = LabelDefaultValue
			}
		}
	}

	return values
}

func (n *Event) getEvent(address *AddressEvent) error {
	var err error

	defer func() {
		if err != nil {
			n.EventError.WithLabelValues(n.getLabelValues(address)...).Inc()
		}
	}()

	currentBlockStr, err := n.client.ETHGetBlockNumber()
	if err != nil {
		return err
	}
	currentBlock64 := hexStringToInt64(currentBlockStr)
	nextStartBlock := address.LastKnownBlock
	if nextStartBlock == 0 {
		nextStartBlock = currentBlock64 - int64(n.BlockIncrement)
		if nextStartBlock < 0 {
			nextStartBlock = 0
		}
	}

	nextEndBlock := nextStartBlock + int64(n.BlockIncrement)
	if nextEndBlock > currentBlock64 {
		nextEndBlock = currentBlock64
	}

	logs, err := n.client.ETHGetEvent(
		address.Contract,
		address.Topic,
		int64ToHexString(nextStartBlock),
		int64ToHexString(nextEndBlock),
	)

	if err != nil {
		return err
	}

	if len(logs) == 0 {
		address.LastKnownBlock = nextEndBlock
		return nil
	}

	if len(logs) > 0 {
		// Get the last log entry
		lastLog := logs[len(logs)-1]
		eventBlock := lastLog.BlockNumber

		n.EventBlock.WithLabelValues(n.getLabelValues(address)...).Set(hexStringToFloat64(eventBlock))
		address.LastKnownBlock = currentBlock64
	}

	return nil
}
