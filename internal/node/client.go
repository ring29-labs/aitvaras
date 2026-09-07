package node

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ring29-labs/aitvaras/internal/model"
)

type Client struct {
	CoreURL string
	Token   string
	HTTP    *http.Client
}

func (c Client) Register(ctx context.Context, registration model.RegisterNodeRequest) (model.Node, error) {
	body, err := json.Marshal(registration)
	if err != nil {
		return model.Node{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.CoreURL, "/")+"/v1/nodes", bytes.NewReader(body))
	if err != nil {
		return model.Node{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return model.Node{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return model.Node{}, fmt.Errorf("control registration returned %s: %s", resp.Status, strings.TrimSpace(string(message)))
	}
	var registered model.Node
	if err := json.NewDecoder(resp.Body).Decode(&registered); err != nil {
		return model.Node{}, err
	}
	return registered, nil
}
