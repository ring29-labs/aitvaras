package budget

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func policy() Policy {
	return Policy{ID: "p1", ClientID: "bot", WalletID: "wallet", Network: "eip155:84532", Asset: "USDC", TradeLimit: "100", GasLimitWei: "1000"}
}
func request() Request {
	return Request{ID: "r1", PolicyID: "p1", WalletID: "wallet", Network: "eip155:84532", Asset: "USDC", Amount: "10", GasLimit: "10", MaxFeePerGas: "2"}
}
func ledger(t *testing.T) *Ledger {
	t.Helper()
	l, err := Open(filepath.Join(t.TempDir(), "budget.journal"), policy(), true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l
}

func TestReplayAndConflict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "budget.journal")
	l, err := Open(path, policy(), true)
	if err != nil {
		t.Fatal(err)
	}
	want, err := l.Reserve(request())
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	l, err = Open(path, policy(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	got, err := l.Reserve(request())
	if err != nil || got != want {
		t.Fatalf("replay: %+v %v", got, err)
	}
	got, err = l.Get("r1")
	if err != nil || got != want {
		t.Fatalf("lookup: %+v %v", got, err)
	}
	req := request()
	req.Amount = "11"
	if _, err := l.Reserve(req); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflict: %v", err)
	}
	s, _ := l.Snapshot()
	if s.Reservations != 1 || s.TradeReserved != "10" || s.GasReservedWei != "20" {
		t.Fatalf("double reservation: %+v", s)
	}
}

func TestDenialsAreAtomic(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Request)
		want   error
	}{
		{"trade cap", func(r *Request) { r.Amount = "101" }, ErrDenied},
		{"gas cap", func(r *Request) { r.MaxFeePerGas = "101" }, ErrDenied},
		{"policy", func(r *Request) { r.PolicyID = "p2" }, ErrDenied},
		{"wallet", func(r *Request) { r.WalletID = "other" }, ErrDenied},
		{"network", func(r *Request) { r.Network = "eip155:1" }, ErrDenied},
		{"asset", func(r *Request) { r.Asset = "ETH" }, ErrDenied},
		{"decimal", func(r *Request) { r.Amount = "1.2" }, ErrInvalid},
		{"negative", func(r *Request) { r.Amount = "-1" }, ErrInvalid},
		{"zero", func(r *Request) { r.Amount = "0" }, ErrInvalid},
		{"leading zero", func(r *Request) { r.Amount = "01" }, ErrInvalid},
		{"oversized", func(r *Request) { r.MaxFeePerGas = strings.Repeat("9", 79) }, ErrInvalid},
		{"gas uint64", func(r *Request) { r.GasLimit = "18446744073709551616" }, ErrInvalid},
		{"empty id", func(r *Request) { r.ID = "" }, ErrInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := ledger(t)
			req := request()
			tc.change(&req)
			if _, err := l.Reserve(req); !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			s, _ := l.Snapshot()
			if s.Reservations != 0 || s.TradeReserved != "0" || s.GasReservedWei != "0" {
				t.Fatalf("mutated on denial: %+v", s)
			}
		})
	}
}

func TestConcurrentBudgetCeiling(t *testing.T) {
	l := ledger(t)
	var wg sync.WaitGroup
	var accepted atomic.Int32
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := request()
			r.ID = fmt.Sprintf("r%d", i)
			if _, err := l.Reserve(r); err == nil {
				accepted.Add(1)
			} else if !errors.Is(err, ErrDenied) {
				t.Errorf("reserve: %v", err)
			}
		}(i)
	}
	wg.Wait()
	s, _ := l.Snapshot()
	if accepted.Load() != 10 || s.TradeRemaining != "0" || s.GasReservedWei != "200" {
		t.Fatalf("overspend: %d %+v", accepted.Load(), s)
	}
}

func TestConcurrentRetryOnlyReservesOnce(t *testing.T) {
	l := ledger(t)
	var wg sync.WaitGroup
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := l.Reserve(request()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	s, _ := l.Snapshot()
	if s.Reservations != 1 || s.TradeReserved != "10" {
		t.Fatalf("duplicate: %+v", s)
	}
}

func TestJournalFailClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "budget.journal")
	if _, err := Open(path, policy(), false); err == nil {
		t.Fatal("missing journal accepted")
	}
	l, err := Open(path, policy(), true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path, policy(), true); err == nil {
		t.Fatal("init overwrote existing journal")
	}
	if _, err := Open(path, policy(), false); err == nil {
		t.Fatal("second owner accepted")
	}
	_ = l.Close()
	p := policy()
	p.TradeLimit = "200"
	if _, err := Open(path, p, false); err == nil {
		t.Fatal("changed policy accepted")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(`{"request_id":`)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if _, err := Open(path, policy(), false); err == nil {
		t.Fatal("partial journal accepted")
	}
}

func TestStorageFailurePoisonsLedger(t *testing.T) {
	l := ledger(t)
	_ = l.file.Close() // Inject a write failure without marking the ledger closed.
	if _, err := l.Reserve(request()); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err := l.Snapshot(); !errors.Is(err, ErrUnavailable) {
		t.Fatal("continued after uncertain write")
	}
	if len(l.receipts) != 0 {
		t.Fatal("committed failed write")
	}
}

func TestDecodeRejectsAmbiguousJSON(t *testing.T) {
	for _, raw := range []string{`{"amount":"1","amount":"2"}`, `{"amount":"1","Amount":"2"}`, `{"extra":true}`, `{} {}`, `{"amount":2}`, `{"amount":`} {
		var r Request
		if err := Decode(strings.NewReader(raw), &r); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
}
