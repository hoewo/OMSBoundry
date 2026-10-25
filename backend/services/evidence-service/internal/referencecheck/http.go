package referencecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL *url.URL
	token   string
	client  *http.Client
}

func New(rawURL, token string, timeout time.Duration) (*Client, error) {
	base, err := url.Parse(strings.TrimRight(strings.TrimSpace(rawURL), "/"))
	if err != nil || !base.IsAbs() || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, fmt.Errorf("reference protection URL is invalid")
	}
	if len(strings.TrimSpace(token)) < 24 || timeout <= 0 {
		return nil, fmt.Errorf("reference protection credential is invalid")
	}
	return &Client{baseURL: base, token: token, client: &http.Client{Timeout: timeout}}, nil
}

func (c *Client) IsReferenced(ctx context.Context, tenantID, fileID string, version int) (bool, error) {
	path := "/api/v1/references/evidence-files/" + url.PathEscape(fileID) + "/versions/" + strconv.Itoa(version) + "/usage"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL.ResolveReference(&url.URL{Path: path}).String(), nil)
	if err != nil {
		return false, err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	response, err := c.client.Do(request)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return false, fmt.Errorf("reference owner returned HTTP %d", response.StatusCode)
	}
	var value struct {
		TenantID   string `json:"tenant_id"`
		FileID     string `json:"file_id"`
		Version    int    `json:"version"`
		Referenced bool   `json:"referenced"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&value); err != nil || value.TenantID != tenantID || value.FileID != fileID || value.Version != version {
		return false, fmt.Errorf("reference owner returned an invalid response")
	}
	var trailing any
	if err = decoder.Decode(&trailing); err != io.EOF {
		return false, fmt.Errorf("reference owner returned trailing data")
	}
	return value.Referenced, nil
}
