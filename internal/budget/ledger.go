// Package budget implements keyless reservation accounting, not transaction authorization.
package budget

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
)

var (
	ErrInvalid     = errors.New("invalid reservation")
	ErrDenied      = errors.New("policy denied")
	ErrConflict    = errors.New("request ID conflict")
	ErrNotFound    = errors.New("reservation not found")
	ErrUnavailable = errors.New("ledger unavailable; operator recovery required")
	idPattern      = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)
	uintPattern    = regexp.MustCompile(`^(0|[1-9][0-9]{0,77})$`)
	networkPattern = regexp.MustCompile(`^eip155:[1-9][0-9]{0,19}$`)
)

const maxRecords = 10000

type Policy struct {
	ID          string `json:"policy_id"`
	ClientID    string `json:"client_id"`
	WalletID    string `json:"wallet_id"`
	Network     string `json:"network"`
	Asset       string `json:"asset"`
	TradeLimit  string `json:"trade_limit"`
	GasLimitWei string `json:"gas_limit_wei"`
}

type Request struct {
	ID           string `json:"request_id"`
	PolicyID     string `json:"policy_id"`
	WalletID     string `json:"wallet_id"`
	Network      string `json:"network"`
	Asset        string `json:"asset"`
	Amount       string `json:"amount"`
	GasLimit     string `json:"gas_limit"`
	MaxFeePerGas string `json:"max_fee_per_gas"`
}

type Receipt struct {
	Request        Request `json:"request"`
	ClientID       string  `json:"client_id"`
	Digest         string  `json:"request_digest"`
	Status         string  `json:"status"`
	GasReservedWei string  `json:"gas_reserved_wei"`
}

type Snapshot struct {
	Policy          Policy `json:"policy"`
	TradeReserved   string `json:"trade_reserved"`
	TradeRemaining  string `json:"trade_remaining"`
	GasReservedWei  string `json:"gas_reserved_wei"`
	GasRemainingWei string `json:"gas_remaining_wei"`
	Reservations    int    `json:"reservations"`
}

type Ledger struct {
	mu         sync.Mutex
	file       *os.File
	policy     Policy
	receipts   map[string]Receipt
	trade, gas *big.Int
	failed     bool
}

func number(value string) (*big.Int, bool) {
	if !uintPattern.MatchString(value) {
		return nil, false
	}
	n, ok := new(big.Int).SetString(value, 10)
	return n, ok && n.BitLen() <= 256
}

func (p Policy) Validate() error {
	for _, value := range []string{p.ID, p.ClientID, p.WalletID, p.Asset} {
		if !idPattern.MatchString(value) {
			return fmt.Errorf("invalid policy identifier")
		}
	}
	if !networkPattern.MatchString(p.Network) {
		return fmt.Errorf("invalid EVM network")
	}
	for _, value := range []string{p.TradeLimit, p.GasLimitWei} {
		if n, ok := number(value); !ok || n.Sign() <= 0 {
			return fmt.Errorf("policy limits must be positive uint256 strings")
		}
	}
	return nil
}

// Decode rejects unknown fields and trailing JSON values. Inputs must be size-bounded by callers.
func Decode(r io.Reader, dst any) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	check := json.NewDecoder(bytes.NewReader(data))
	if err := uniqueKeys(check, 0); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("expected exactly one JSON value")
	}
	return nil
}

// encoding/json otherwise accepts duplicate and case-insensitive struct keys.
func uniqueKeys(d *json.Decoder, depth int) error {
	if depth > 16 {
		return errors.New("JSON nesting limit")
	}
	t, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	keys := make(map[string]bool)
	for d.More() {
		if delim == '{' {
			t, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := t.(string)
			if !ok || keys[key] || key != strings.ToLower(key) {
				return errors.New("duplicate or noncanonical JSON key")
			}
			keys[key] = true
		}
		if err := uniqueKeys(d, depth+1); err != nil {
			return err
		}
	}
	_, err = d.Token()
	return err
}

// Open requires an existing journal unless initialize is explicit. It never overwrites.
// The lock is advisory and local: copied files and other hosts are not fenced.
func Open(path string, policy Policy, initialize bool) (*Ledger, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	flags := os.O_RDWR | os.O_APPEND
	if initialize {
		flags |= os.O_CREATE | os.O_EXCL
	}
	f, err := os.OpenFile(path, flags, 0600)
	if err != nil {
		return nil, err
	}
	good := false
	defer func() {
		if !good {
			_ = f.Close()
		}
	}()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, fmt.Errorf("journal already owned: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 16<<20 {
		return nil, errors.New("invalid journal file or size")
	}
	l := &Ledger{file: f, policy: policy, receipts: make(map[string]Receipt), trade: new(big.Int), gas: new(big.Int)}
	header, _ := json.Marshal(struct {
		Version int    `json:"version"`
		Policy  Policy `json:"policy"`
	}{1, policy})
	if initialize {
		if err := l.append(header); err != nil {
			return nil, err
		}
		dir, err := os.Open(filepath.Dir(path))
		if err != nil {
			return nil, err
		}
		err = dir.Sync()
		_ = dir.Close()
		if err != nil {
			return nil, err
		}
	} else {
		r := bufio.NewReader(io.LimitReader(f, 16<<20))
		line, err := r.ReadBytes('\n')
		if err != nil || !bytes.Equal(bytes.TrimSuffix(line, []byte{'\n'}), header) {
			return nil, errors.New("missing journal header or changed policy")
		}
		for {
			line, err = r.ReadBytes('\n')
			if err == io.EOF && len(line) == 0 {
				break
			}
			if err != nil || len(line) > 4096 {
				return nil, errors.New("incomplete or oversized journal record")
			}
			var req Request
			if err := Decode(bytes.NewReader(line), &req); err != nil {
				return nil, errors.New("invalid journal JSON")
			}
			if _, exists := l.receipts[req.ID]; exists {
				return nil, errors.New("duplicate journal record")
			}
			receipt, trade, gas, err := l.prepare(req)
			if err != nil || len(l.receipts) >= maxRecords {
				return nil, errors.New("journal violates policy or capacity")
			}
			l.commit(receipt, trade, gas)
		}
	}
	good = true
	return l, nil
}

func (l *Ledger) prepare(req Request) (Receipt, *big.Int, *big.Int, error) {
	if !idPattern.MatchString(req.ID) {
		return Receipt{}, nil, nil, ErrInvalid
	}
	amount, amountOK := number(req.Amount)
	gasLimit, gasOK := number(req.GasLimit)
	fee, feeOK := number(req.MaxFeePerGas)
	if !amountOK || !gasOK || !feeOK || amount.Sign() <= 0 || gasLimit.Sign() <= 0 || gasLimit.BitLen() > 64 || fee.Sign() <= 0 {
		return Receipt{}, nil, nil, ErrInvalid
	}
	p := l.policy
	if req.PolicyID != p.ID || req.WalletID != p.WalletID || req.Network != p.Network || req.Asset != p.Asset {
		return Receipt{}, nil, nil, ErrDenied
	}
	gas := new(big.Int).Mul(gasLimit, fee)
	tradeTotal := new(big.Int).Add(l.trade, amount)
	gasTotal := new(big.Int).Add(l.gas, gas)
	tradeCap, _ := number(p.TradeLimit)
	gasCap, _ := number(p.GasLimitWei)
	if tradeTotal.Cmp(tradeCap) > 0 || gasTotal.Cmp(gasCap) > 0 {
		return Receipt{}, nil, nil, ErrDenied
	}
	data, _ := json.Marshal(req)
	digest := sha256.Sum256(data)
	receipt := Receipt{Request: req, ClientID: p.ClientID, Digest: hex.EncodeToString(digest[:]), Status: "reserved_simulation", GasReservedWei: gas.String()}
	return receipt, tradeTotal, gasTotal, nil
}

func (l *Ledger) append(data []byte) error {
	data = append(data, '\n')
	n, err := l.file.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = l.file.Sync()
	}
	if err != nil {
		l.failed = true
		return ErrUnavailable
	}
	return nil
}

func (l *Ledger) commit(receipt Receipt, trade, gas *big.Int) {
	l.receipts[receipt.Request.ID] = receipt
	l.trade, l.gas = trade, gas
}

func (l *Ledger) Reserve(req Request) (Receipt, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.failed {
		return Receipt{}, ErrUnavailable
	}
	if old, ok := l.receipts[req.ID]; ok {
		if old.Request != req {
			return Receipt{}, ErrConflict
		}
		return old, nil
	}
	receipt, trade, gas, err := l.prepare(req)
	if err != nil {
		return Receipt{}, err
	}
	if len(l.receipts) >= maxRecords {
		return Receipt{}, ErrUnavailable
	}
	data, _ := json.Marshal(req)
	if err := l.append(data); err != nil {
		return Receipt{}, err
	}
	l.commit(receipt, trade, gas)
	return receipt, nil
}

func (l *Ledger) Get(id string) (Receipt, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.failed {
		return Receipt{}, ErrUnavailable
	}
	receipt, ok := l.receipts[id]
	if !ok {
		return Receipt{}, ErrNotFound
	}
	return receipt, nil
}

func (l *Ledger) Snapshot() (Snapshot, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.failed {
		return Snapshot{}, ErrUnavailable
	}
	tradeCap, _ := number(l.policy.TradeLimit)
	gasCap, _ := number(l.policy.GasLimitWei)
	return Snapshot{Policy: l.policy, TradeReserved: l.trade.String(), TradeRemaining: tradeCap.Sub(tradeCap, l.trade).String(), GasReservedWei: l.gas.String(), GasRemainingWei: gasCap.Sub(gasCap, l.gas).String(), Reservations: len(l.receipts)}, nil
}

func (l *Ledger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.failed = true
	return l.file.Close()
}
