package exporter

import (
	"testing"

	"github.com/ethpandaops/ethereum-address-metrics-exporter/pkg/exporter/jobs"
)

func TestValidateRequiresStateFileForTransactionInputPrefix(t *testing.T) {
	cfg := &Config{
		Addresses: Addresses{
			TransactionInputPrefix: []*jobs.AddressTransactionInputPrefix{
				{
					Name:        "batch-poster",
					Contract:    "0x00000000000000000000000000000000000000aa",
					InputPrefix: "0x30",
				},
			},
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateAllowsTransactionInputPrefixWithStateFile(t *testing.T) {
	cfg := &Config{
		GlobalConfig: GlobalConfig{
			StateFile: "./state.json",
		},
		Addresses: Addresses{
			TransactionInputPrefix: []*jobs.AddressTransactionInputPrefix{
				{
					Name:        "batch-poster",
					Contract:    "0x00000000000000000000000000000000000000aa",
					InputPrefix: "0x30",
				},
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
