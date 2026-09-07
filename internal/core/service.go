package core

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ring29-labs/aitvaras/internal/model"
)

var (
	caip2Pattern  = regexp.MustCompile(`^[a-z0-9]+:[A-Za-z0-9_-]+$`)
	amountPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]+)?$`)
)

var ErrNotFound = errors.New("not found")

type Service struct {
	mu      sync.RWMutex
	nodes   map[string]model.Node
	intents map[string]model.Intent
	now     func() time.Time
}

func New() *Service {
	return &Service{
		nodes:   make(map[string]model.Node),
		intents: make(map[string]model.Intent),
		now:     func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) RegisterNode(req model.RegisterNodeRequest) (model.Node, error) {
	if strings.TrimSpace(req.ID) == "" || strings.TrimSpace(req.TenantID) == "" {
		return model.Node{}, errors.New("id and tenant_id are required")
	}
	if err := validHTTPURL(req.PublicURL, false); err != nil {
		return model.Node{}, fmt.Errorf("public_url: %w", err)
	}
	if len(req.Chains) == 0 {
		return model.Node{}, errors.New("at least one chain is required")
	}
	for _, chain := range req.Chains {
		if !caip2Pattern.MatchString(chain) {
			return model.Node{}, fmt.Errorf("invalid CAIP-2 chain %q", chain)
		}
	}

	node := model.Node{
		ID: req.ID, TenantID: req.TenantID, PublicURL: req.PublicURL,
		Chains: append([]string(nil), req.Chains...), Capabilities: append([]string(nil), req.Capabilities...),
		Status: "registered", RegisteredAt: s.now(),
	}
	s.mu.Lock()
	s.nodes[node.ID] = node
	s.mu.Unlock()
	return node, nil
}

func (s *Service) ListNodes() []model.Node {
	s.mu.RLock()
	result := make([]model.Node, 0, len(s.nodes))
	for _, node := range s.nodes {
		result = append(result, node)
	}
	s.mu.RUnlock()
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (s *Service) CreateIntent(req model.CreateIntentRequest) (model.Intent, error) {
	if req.Kind != "x402_payment" {
		return model.Intent{}, errors.New("kind must be x402_payment")
	}
	if strings.TrimSpace(req.TenantID) == "" || strings.TrimSpace(req.AgentID) == "" || strings.TrimSpace(req.WalletID) == "" {
		return model.Intent{}, errors.New("tenant_id, agent_id, and wallet_id are required")
	}
	if !caip2Pattern.MatchString(req.Payment.Network) {
		return model.Intent{}, errors.New("payment.network must be a CAIP-2 identifier")
	}
	if strings.TrimSpace(req.Payment.Asset) == "" || strings.TrimSpace(req.Payment.PayTo) == "" {
		return model.Intent{}, errors.New("payment.asset and payment.pay_to are required")
	}
	amount, parsed := new(big.Rat).SetString(req.Payment.Amount)
	if !amountPattern.MatchString(req.Payment.Amount) || !parsed || amount.Sign() <= 0 {
		return model.Intent{}, errors.New("payment.amount must be a positive decimal string")
	}
	if err := validHTTPURL(req.Payment.Resource, true); err != nil {
		return model.Intent{}, fmt.Errorf("payment.resource: %w", err)
	}

	id, err := newID("int")
	if err != nil {
		return model.Intent{}, fmt.Errorf("create intent id: %w", err)
	}
	intent := model.Intent{
		ID: id, TenantID: req.TenantID, AgentID: req.AgentID, WalletID: req.WalletID,
		Kind: req.Kind, Payment: req.Payment, Status: "pending_policy", CreatedAt: s.now(),
	}
	s.mu.Lock()
	s.intents[intent.ID] = intent
	s.mu.Unlock()
	return intent, nil
}

func (s *Service) GetIntent(id string) (model.Intent, error) {
	s.mu.RLock()
	intent, ok := s.intents[id]
	s.mu.RUnlock()
	if !ok {
		return model.Intent{}, ErrNotFound
	}
	return intent, nil
}

func validHTTPURL(raw string, allowHTTPSOnly bool) error {
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.Host == "" {
		return errors.New("must be an absolute HTTP URL")
	}
	if allowHTTPSOnly && u.Scheme != "https" {
		return errors.New("must use https")
	}
	if !allowHTTPSOnly && u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("must use http or https")
	}
	return nil
}

func newID(prefix string) (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(buf), nil
}
