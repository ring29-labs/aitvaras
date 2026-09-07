package budgetapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ring29-labs/aitvaras/internal/budget"
)

func TestClientControlNodeFlow(t *testing.T) {
	p := budget.Policy{ID: "p1", ClientID: "bot", WalletID: "w1", Network: "eip155:84532", Asset: "USDC", TradeLimit: "20", GasLimitWei: "100"}
	l, err := budget.Open(filepath.Join(t.TempDir(), "budget.journal"), p, true)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	node := httptest.NewServer(Node(l, "node-only"))
	defer node.Close()
	handler, err := Control(node.URL, "bot-only", "node-only")
	if err != nil {
		t.Fatal(err)
	}
	control := httptest.NewServer(handler)
	defer control.Close()
	req := budget.Request{ID: "r1", PolicyID: "p1", WalletID: "w1", Network: "eip155:84532", Asset: "USDC", Amount: "10", GasLimit: "10", MaxFeePerGas: "2"}
	data, _ := json.Marshal(req)
	call := func(base, method, path, token string, body []byte, want int) []byte {
		t.Helper()
		r, _ := http.NewRequest(method, base+path, bytes.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var b bytes.Buffer
		_, _ = b.ReadFrom(resp.Body)
		if resp.StatusCode != want {
			t.Fatalf("%s %s: %d want %d: %s", method, path, resp.StatusCode, want, b.String())
		}
		return b.Bytes()
	}
	path := "/v1/budget/reservations"
	call(control.URL, "POST", path, "wrong", data, 401)
	call(control.URL, "POST", path, "node-only", data, 401)
	call(node.URL, "POST", path, "bot-only", data, 401)
	first := call(control.URL, "POST", path, "bot-only", data, 200)
	again := call(control.URL, "POST", path, "bot-only", data, 200)
	if !bytes.Equal(first, again) {
		t.Fatal("retry receipt changed")
	}
	got := call(control.URL, "GET", path+"/r1", "bot-only", nil, 200)
	if !bytes.Equal(first, got) {
		t.Fatal("lookup receipt changed")
	}
	call(control.URL, "GET", path+"/missing", "bot-only", nil, 404)
	req.Amount = "11"
	data, _ = json.Marshal(req)
	call(control.URL, "POST", path, "bot-only", data, 409)
	req.ID = "r2"
	data, _ = json.Marshal(req)
	call(control.URL, "POST", path, "bot-only", data, 403)
	for _, bad := range []string{`{"unknown":1}`, `{"amount":"1","amount":"2"}`, `{} {}`, `{"amount":1}`, strings.Repeat("x", 9000)} {
		call(node.URL, "POST", path, "node-only", []byte(bad), 400)
	}
	call(control.URL, "DELETE", "/v1/budget", "bot-only", nil, 405)
	var snap budget.Snapshot
	if err := json.Unmarshal(call(control.URL, "GET", "/v1/budget", "bot-only", nil, 200), &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Reservations != 1 || snap.TradeReserved != "10" || snap.GasReservedWei != "20" {
		t.Fatalf("bad counters: %+v", snap)
	}
	node.Close()
	call(control.URL, "POST", path, "bot-only", data, 502)
}

func TestRoutingRejectsUntrustedDestination(t *testing.T) {
	for _, dest := range []string{"https://example.com", "http://localhost:8082", "http://127.0.0.1/path", "http://user@127.0.0.1", "http://127.0.0.1?x=1", ""} {
		if _, err := Control(dest, "bot", "node"); err == nil {
			t.Errorf("accepted %s", dest)
		}
	}
	for _, tokens := range [][2]string{{"", "node"}, {"bot", ""}, {"same", "same"}} {
		if _, err := Control("http://127.0.0.1:8082", tokens[0], tokens[1]); err == nil {
			t.Fatal("invalid credentials accepted")
		}
	}
}
