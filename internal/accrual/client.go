package accrual

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Cyclov/gophermart/internal/model"
	"github.com/go-resty/resty/v2"
)

var (
	ErrPending   = errors.New("order not found by accrual")
	ErrTemporary = errors.New("accrual temporarily unavailable")
	ErrResponse  = errors.New("invalid accrual response")
)

type RateLimitError struct{ Delay time.Duration }

func (e *RateLimitError) Error() string { return "accrual rate limit exceeded" }

type Client struct {
	http *resty.Client
	base string
}

func NewClient(base string) *Client {
	client := resty.New().SetTimeout(5 * time.Second).SetRetryCount(0).SetRedirectPolicy(resty.NoRedirectPolicy())
	return &Client{http: client, base: strings.TrimRight(base, "/")}
}

func (c *Client) Get(ctx context.Context, number string) (model.Accrual, error) {
	response, err := c.http.R().SetContext(ctx).SetDoNotParseResponse(true).Get(c.base + "/api/orders/" + url.PathEscape(number))
	if err != nil {
		return model.Accrual{}, ErrTemporary
	}
	defer response.RawBody().Close()
	switch response.StatusCode() {
	case http.StatusNoContent:
		return model.Accrual{}, ErrPending
	case http.StatusTooManyRequests:
		return model.Accrual{}, &RateLimitError{Delay: retryAfter(response.Header().Get("Retry-After"), time.Now())}
	case http.StatusOK:
		body, err := io.ReadAll(io.LimitReader(response.RawBody(), (1<<20)+1))
		if err != nil {
			return model.Accrual{}, ErrTemporary
		}
		if len(body) > 1<<20 {
			return model.Accrual{}, ErrResponse
		}
		return decode(body, number)
	default:
		return model.Accrual{}, ErrTemporary
	}
}

func decode(body []byte, number string) (model.Accrual, error) {
	var result model.Accrual
	if err := json.Unmarshal(body, &result); err != nil || result.Number != number {
		return model.Accrual{}, ErrResponse
	}
	switch result.Status {
	case "REGISTERED", "PROCESSING", "INVALID", "PROCESSED":
	default:
		return model.Accrual{}, ErrResponse
	}
	if result.Amount != nil {
		if _, err := model.Amount(*result.Amount, false); err != nil {
			return model.Accrual{}, ErrResponse
		}
	}
	return result, nil
}

func retryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 && seconds <= int64((1<<63-1)/int64(time.Second)) {
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(value); err == nil {
		if delay := date.Sub(now); delay > 0 {
			return delay
		}
		return 0
	}
	return time.Second
}
