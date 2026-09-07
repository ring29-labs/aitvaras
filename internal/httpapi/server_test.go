package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ring29-labs/aitvaras/internal/core"
	"github.com/ring29-labs/aitvaras/internal/httpapi"
	"github.com/ring29-labs/aitvaras/internal/model"
)

const testToken = "test-token"

func TestAuthorizationRequired(t *testing.T) {
	recorder := httptest.NewRecorder()
	httpapi.New(core.New(), testToken).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/nodes", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestRegisterAndListNode(t *testing.T) {
	handler := httpapi.New(core.New(), testToken)
	body := `{"id":"node-1","tenant_id":"tenant-1","public_url":"http://127.0.0.1:8081","chains":["eip155:84532"],"capabilities":["registration_only"]}`
	recorder := do(t, handler, http.MethodPost, "/v1/nodes", body)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("register got %d: %s", recorder.Code, recorder.Body.String())
	}

	recorder = do(t, handler, http.MethodGet, "/v1/nodes", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("list got %d", recorder.Code)
	}
	var response struct {
		Nodes []model.Node `json:"nodes"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Nodes) != 1 || response.Nodes[0].ID != "node-1" {
		t.Fatalf("unexpected nodes: %#v", response.Nodes)
	}
}

func TestCreateAndGetIntent(t *testing.T) {
	handler := httpapi.New(core.New(), testToken)
	body := `{"tenant_id":"tenant-1","agent_id":"agent-1","wallet_id":"wallet-1","kind":"x402_payment","payment":{"network":"eip155:84532","asset":"USDC","amount":"0.10","pay_to":"0x1111111111111111111111111111111111111111","resource":"https://example.com/paid"}}`
	recorder := do(t, handler, http.MethodPost, "/v1/intents", body)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("create got %d: %s", recorder.Code, recorder.Body.String())
	}
	var created model.Intent
	if err := json.Unmarshal(recorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Status != "pending_policy" {
		t.Fatalf("unexpected status %q", created.Status)
	}

	recorder = do(t, handler, http.MethodGet, "/v1/intents/"+created.ID, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("get got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestRejectsBlindOrMalformedIntent(t *testing.T) {
	handler := httpapi.New(core.New(), testToken)
	tests := []string{
		`{"tenant_id":"t","agent_id":"a","wallet_id":"w","kind":"raw_transaction","payment":{}}`,
		`{"tenant_id":"t","agent_id":"a","wallet_id":"w","kind":"x402_payment","payment":{"network":"84532","asset":"USDC","amount":"1","pay_to":"x","resource":"https://example.com"}}`,
		`{"tenant_id":"t","agent_id":"a","wallet_id":"w","kind":"x402_payment","payment":{"network":"eip155:84532","asset":"USDC","amount":"0","pay_to":"x","resource":"https://example.com"}}`,
		`{"tenant_id":"t","agent_id":"a","wallet_id":"w","kind":"x402_payment","payment":{"network":"eip155:84532","asset":"USDC","amount":"0.000","pay_to":"x","resource":"https://example.com"}}`,
	}
	for _, body := range tests {
		recorder := do(t, handler, http.MethodPost, "/v1/intents", body)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("got %d for %s: %s", recorder.Code, body, recorder.Body.String())
		}
	}
}

func do(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testToken)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}
