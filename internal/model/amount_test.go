package model

import (
	"encoding/json"
	"testing"
)

func TestAmount(t *testing.T) {
	for _, tt := range []struct {
		value           string
		positive, valid bool
	}{
		{"0", false, true}, {"0", true, false}, {"0.0000001", true, true}, {"1e-3", true, true}, {"-1", false, false}, {"NaN", false, false}, {"01", false, false}, {"null", false, false}, {"1/2", false, false}, {"1e1001", false, false}, {"", false, false},
	} {
		_, err := Amount(json.Number(tt.value), tt.positive)
		if (err == nil) != tt.valid {
			t.Errorf("%s: %v", tt.value, err)
		}
	}
}
