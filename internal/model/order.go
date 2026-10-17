package model

import (
	"encoding/json"
	"time"
)

type Order struct {
	Number     string       `json:"number"`
	Status     string       `json:"status"`
	Accrual    *json.Number `json:"accrual,omitempty"`
	UploadedAt time.Time    `json:"uploaded_at"`
}
