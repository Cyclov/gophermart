package model

import (
	"encoding/json"
	"errors"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

var decimalPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

func Amount(value json.Number, positive bool) (string, error) {
	text := value.String()
	if len(text) == 0 || len(text) > 4096 || !decimalPattern.MatchString(text) {
		return "", errors.New("invalid amount")
	}
	if index := strings.IndexAny(text, "eE"); index >= 0 {
		exponent, err := strconv.Atoi(text[index+1:])
		if err != nil || exponent > 1000 || exponent < -1000 {
			return "", errors.New("amount exponent out of range")
		}
	}
	number, ok := new(big.Rat).SetString(text)
	if !ok || number.Sign() < 0 || (positive && number.Sign() == 0) {
		return "", errors.New("invalid amount")
	}
	return text, nil
}

type Accrual struct {
	Number string       `json:"order"`
	Status string       `json:"status"`
	Amount *json.Number `json:"accrual,omitempty"`
}
