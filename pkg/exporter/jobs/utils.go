package jobs

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

const (
	LabelAddress       string = "address"
	LabelBytesArgIndex string = "bytes_arg_index"
	LabelContract      string = "contract"
	LabelDefaultValue  string = ""
	LabelFrom          string = "from"
	LabelInputPrefix   string = "input_prefix"
	LabelMatched       string = "matched"
	LabelMethodPrefix  string = "method_prefix"
	LabelName          string = "name"
	LabelSymbol        string = "symbol"
	LabelTo            string = "to"
	LabelTokenID       string = "token_id"
	LabelTopic         string = "topic"
)

func hexStringToFloat64(hexStr string) float64 {
	f := new(big.Float)
	f.SetString(hexStr)
	balance, _ := f.Float64()

	return balance
}

func hexStringToString(hexStr string) (string, error) {
	bs, err := hex.DecodeString(hexStr[2:])
	if err != nil {
		return "", err
	}

	// split on end of transmission
	splitTransmission := bytes.Split(bs, []byte{4})
	last := splitTransmission[len(splitTransmission)-1]

	// split on end of text
	splitText := bytes.Split(last, []byte{3})
	last = splitText[len(splitText)-1]

	// trim null bytes and spaces
	last = bytes.Trim(last, "\x00")
	last = bytes.TrimSpace(last)

	return string(last), nil
}

func hexStringToInt64(hexStr string) int64 {

	f := new(big.Int)
	f.SetString(hexStr[2:], 16)
	return f.Int64()
}

func int64ToHexString(i int64) string {
	f := new(big.Int)
	f.SetInt64(i)
	return "0x" + f.Text(16)
}

func normalizeHexPrefix(prefix string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(prefix))
	if normalized == "" {
		return "", fmt.Errorf("empty hex prefix")
	}

	normalized = strings.TrimPrefix(normalized, "0x")
	if normalized == "" {
		return "", fmt.Errorf("empty hex prefix")
	}

	for _, c := range normalized {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return "", fmt.Errorf("invalid hex prefix %q", prefix)
		}
	}

	return "0x" + normalized, nil
}

func hasHexPrefix(input string, prefix string) bool {
	normalizedInput := strings.ToLower(strings.TrimSpace(input))
	if !strings.HasPrefix(normalizedInput, "0x") {
		normalizedInput = "0x" + normalizedInput
	}

	return strings.HasPrefix(normalizedInput, prefix)
}

func sameHexAddress(left string, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

func decodeABIBytesArgument(calldata string, argIndex int) (string, error) {
	if argIndex < 0 {
		return "", fmt.Errorf("argument index must be greater than or equal to 0")
	}

	normalized := strings.ToLower(strings.TrimSpace(calldata))
	normalized = strings.TrimPrefix(normalized, "0x")
	if len(normalized)%2 != 0 {
		return "", fmt.Errorf("calldata has odd hex length")
	}

	const (
		methodSelectorHexLength = 8
		abiWordHexLength        = 64
	)

	if len(normalized) < methodSelectorHexLength {
		return "", fmt.Errorf("calldata shorter than method selector")
	}

	argsStart := methodSelectorHexLength
	headStart := argsStart + argIndex*abiWordHexLength
	headEnd := headStart + abiWordHexLength
	if headEnd > len(normalized) {
		return "", fmt.Errorf("calldata missing ABI head slot for argument %d", argIndex)
	}

	offset, err := hexWordToInt(normalized[headStart:headEnd])
	if err != nil {
		return "", fmt.Errorf("invalid ABI offset for argument %d: %w", argIndex, err)
	}

	lengthStart := argsStart + offset*2
	lengthEnd := lengthStart + abiWordHexLength
	if lengthEnd > len(normalized) {
		return "", fmt.Errorf("calldata missing ABI bytes length for argument %d", argIndex)
	}

	length, err := hexWordToInt(normalized[lengthStart:lengthEnd])
	if err != nil {
		return "", fmt.Errorf("invalid ABI bytes length for argument %d: %w", argIndex, err)
	}

	dataStart := lengthEnd
	dataEnd := dataStart + length*2
	if dataEnd > len(normalized) {
		return "", fmt.Errorf("calldata missing ABI bytes data for argument %d", argIndex)
	}

	return "0x" + normalized[dataStart:dataEnd], nil
}

func hexWordToInt(word string) (int, error) {
	value := new(big.Int)
	if _, ok := value.SetString(word, 16); !ok {
		return 0, fmt.Errorf("invalid hex word %q", word)
	}

	if !value.IsInt64() {
		return 0, fmt.Errorf("hex word too large")
	}

	maxInt := int64(int(^uint(0) >> 1))
	valueInt64 := value.Int64()
	if valueInt64 > maxInt {
		return 0, fmt.Errorf("hex word too large")
	}

	return int(valueInt64), nil
}
