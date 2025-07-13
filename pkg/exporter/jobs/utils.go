package jobs

import (
	"bytes"
	"encoding/hex"
	"math/big"
)

const (
	LabelAddress      string = "address"
	LabelContract     string = "contract"
	LabelDefaultValue string = ""
	LabelFrom         string = "from"
	LabelName         string = "name"
	LabelSymbol       string = "symbol"
	LabelTo           string = "to"
	LabelTokenID      string = "token_id"
	LabelTopic        string = "topic"
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
