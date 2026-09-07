package model

import "time"

type Node struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	PublicURL    string    `json:"public_url"`
	Chains       []string  `json:"chains"`
	Capabilities []string  `json:"capabilities"`
	Status       string    `json:"status"`
	RegisteredAt time.Time `json:"registered_at"`
}

type RegisterNodeRequest struct {
	ID           string   `json:"id"`
	TenantID     string   `json:"tenant_id"`
	PublicURL    string   `json:"public_url"`
	Chains       []string `json:"chains"`
	Capabilities []string `json:"capabilities"`
}

type Payment struct {
	Network  string `json:"network"`
	Asset    string `json:"asset"`
	Amount   string `json:"amount"`
	PayTo    string `json:"pay_to"`
	Resource string `json:"resource"`
}

type CreateIntentRequest struct {
	TenantID string  `json:"tenant_id"`
	AgentID  string  `json:"agent_id"`
	WalletID string  `json:"wallet_id"`
	Kind     string  `json:"kind"`
	Payment  Payment `json:"payment"`
}

type Intent struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	AgentID   string    `json:"agent_id"`
	WalletID  string    `json:"wallet_id"`
	Kind      string    `json:"kind"`
	Payment   Payment   `json:"payment"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
