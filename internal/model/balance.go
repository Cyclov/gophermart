package model

import (
	"encoding/json"
	"time"
)

type Balance struct {
	Current   json.Number `json:"current"`
	Withdrawn json.Number `json:"withdrawn"`
}
type Withdrawal struct {
	Order       string      `json:"order"`
	Sum         json.Number `json:"sum"`
	ProcessedAt time.Time   `json:"processed_at"`
}
