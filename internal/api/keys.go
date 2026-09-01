package api

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Key is one registered API key record.
type Key struct {
	ID             int64      `json:"id"`
	App            string     `json:"app"`
	Kind           string     `json:"kind"`
	Prefix         string     `json:"prefix"`
	AllowedOrigins []string   `json:"allowed_origins"`
	DailyQuota     int        `json:"daily_quota"`
	UsedToday      int64      `json:"used_today,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
}

// CreateKeyRequest carries parameters to mint a new API key.
type CreateKeyRequest struct {
	App            string   `json:"app"`
	Kind           string   `json:"kind"`
	AllowedOrigins []string `json:"allowed_origins,omitempty"`
	DailyQuota     int      `json:"daily_quota,omitempty"`
}

// CreateKeyResponse returns the created key metadata and the raw one-time token.
type CreateKeyResponse struct {
	Key   Key    `json:"key"`
	Token string `json:"token"`
}

// ListKeysResponse carries a slice of registered API keys.
type ListKeysResponse struct {
	Keys []Key `json:"keys"`
}

// ListKeys returns registered API keys, optionally filtered by application.
func (c *Client) ListKeys(ctx context.Context, app string) ([]Key, error) {
	path := "/api/apikeys"
	if app != "" {
		path += "?app=" + url.QueryEscape(app)
	}
	var out ListKeysResponse
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	return out.Keys, err
}

// CreateKey mints a new secret or public API key.
func (c *Client) CreateKey(ctx context.Context, req CreateKeyRequest) (CreateKeyResponse, error) {
	var out CreateKeyResponse
	err := c.do(ctx, http.MethodPost, "/api/apikeys", req, &out)
	return out, err
}

// RevokeKey revokes an API key by its numeric ID.
func (c *Client) RevokeKey(ctx context.Context, id int64) error {
	path := "/api/apikeys/" + strconv.FormatInt(id, 10)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}
