package network
 
import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
 
	"dmodbus/internal/blockchain"
)
 
func TestAddAndGetPeers(t *testing.T) {
	pn := NewPeerNetwork("Validador_Node_01")
	if len(pn.GetPeers()) != 0 {
		t.Fatal("rede nova deveria começar sem peers")
	}
 
	pn.AddPeer("http://localhost:8081", "Validador_Node_02")
	pn.AddPeer("http://localhost:8082", "Validador_Node_03")
	if got := len(pn.GetPeers()); got != 2 {
		t.Fatalf("esperava 2 peers, obtive %d", got)
	}
 
	// Mesmo endereço não deve duplicar.
	pn.AddPeer("http://localhost:8081", "Validador_Node_02")
	if got := len(pn.GetPeers()); got != 2 {
		t.Fatalf("endereço repetido não deveria duplicar; obtive %d", got)
	}
}
 
// TestQueryPeerMAC sobe um peer falso que responde /internal-query e verifica
// que QueryPeerMAC extrai corretamente o MAC.
func TestQueryPeerMAC(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal-query" {
			http.NotFound(w, r)
			return
		}
		if ip := r.URL.Query().Get("ip"); ip != "172.25.10.52" {
			http.Error(w, "ip inesperado", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(blockchain.DeviceIdentity{
			IP:  "172.25.10.52",
			MAC: "D0:94:66:E2:8E:93",
		})
	}))
	defer srv.Close()
 
	pn := NewPeerNetwork("Validador_Node_01")
	mac, err := pn.QueryPeerMAC(srv.URL, "172.25.10.52")
	if err != nil {
		t.Fatalf("QueryPeerMAC: %v", err)
	}
	if mac != "D0:94:66:E2:8E:93" {
		t.Errorf("MAC = %q, esperado %q", mac, "D0:94:66:E2:8E:93")
	}
}
 
// TestQueryPeerMACNotFound garante que um status diferente de 200 vira erro,
// para que o MAC não seja contabilizado como voto na consulta distribuída.
func TestQueryPeerMACNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "não encontrado", http.StatusNotFound)
	}))
	defer srv.Close()
 
	pn := NewPeerNetwork("Validador_Node_01")
	if _, err := pn.QueryPeerMAC(srv.URL, "10.0.0.1"); err == nil {
		t.Fatal("QueryPeerMAC deveria devolver erro para status != 200")
	}
}
 
// TestBroadcastBlock confirma que o bloco chega ao peer com o envelope correto.
func TestBroadcastBlock(t *testing.T) {
	received := make(chan BlockBroadcast, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/receive-block" {
			http.NotFound(w, r)
			return
		}
		var b BlockBroadcast
		_ = json.NewDecoder(r.Body).Decode(&b)
		received <- b
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
 
	pn := NewPeerNetwork("Validador_Node_01")
	pn.AddPeer(srv.URL, "Validador_Node_02")
 
	block := blockchain.NewBlock(
		blockchain.DeviceIdentity{ID: "SENSOR", IP: "172.25.10.52", MAC: "D0:94:66:E2:8E:93"},
		[]byte{},
		"Validador_Node_01",
	)
	pn.BroadcastBlock(block) // assíncrono: dispara goroutines
 
	select {
	case b := <-received:
		if b.Sender != "Validador_Node_01" {
			t.Errorf("Sender = %q, esperado Validador_Node_01", b.Sender)
		}
		if b.Block == nil || b.Block.Validator != "Validador_Node_01" {
			t.Error("bloco propagado veio incompleto")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: peer não recebeu o bloco propagado")
	}
}