package blockchain
 
import "testing"
 
// newTestChain cria uma ledger isolada em diretório temporário, sem rede
// (broadcaster nil), e garante o fechamento ao fim do teste.
func newTestChain(t *testing.T) *Blockchain {
	t.Helper()
	bc, err := NewBlockchain(t.TempDir(), []string{"Validador_Node_01"}, nil)
	if err != nil {
		t.Fatalf("NewBlockchain: %v", err)
	}
	t.Cleanup(func() { _ = bc.Close() })
	return bc
}
 
func TestQuorum(t *testing.T) {
	// Eq. 1  T = floor(N/2) + 1.
	cases := map[int]int{1: 1, 2: 2, 3: 2, 4: 3, 5: 3}
	for n, want := range cases {
		if got := Quorum(n); got != want {
			t.Errorf("Quorum(%d) = %d, esperado %d", n, got, want)
		}
	}
}
 
func TestGenesisCreated(t *testing.T) {
	bc := newTestChain(t)
	if got := bc.GetBlockCount(); got != 1 {
		t.Fatalf("ledger nova deveria ter só o gênese (1 bloco), tem %d", got)
	}
}
 
func TestAddBlockAndFind(t *testing.T) {
	bc := newTestChain(t)
 
	dev := DeviceIdentity{ID: "SENSOR", IP: "172.25.10.52", MAC: "d0-94-66-e2-8e-93"}
	if err := bc.AddBlock(dev, "Validador_Node_01"); err != nil {
		t.Fatalf("AddBlock: %v", err)
	}
 
	if got := bc.GetBlockCount(); got != 2 {
		t.Fatalf("esperava 2 blocos (gênese + 1), obtive %d", got)
	}
 
	found, ok := bc.FindDeviceByIP("172.25.10.52")
	if !ok {
		t.Fatal("dispositivo não encontrado após AddBlock")
	}

	if found.MAC != "D0:94:66:E2:8E:93" {
		t.Errorf("MAC = %q, esperado normalizado %q", found.MAC, "D0:94:66:E2:8E:93")
	}
}
 
func TestUnauthorizedValidatorRejected(t *testing.T) {
	bc := newTestChain(t)
	dev := DeviceIdentity{ID: "X", IP: "10.0.0.1", MAC: "00:00:00:00:00:01"}
	if err := bc.AddBlock(dev, "Atacante_Node_99"); err == nil {
		t.Fatal("AddBlock deveria recusar validador não autorizado")
	}
}
 
func TestValidateDeviceMAC(t *testing.T) {
	bc := newTestChain(t)
	dev := DeviceIdentity{ID: "SENSOR", IP: "172.25.10.52", MAC: "D0:94:66:E2:8E:93"}
	if err := bc.AddBlock(dev, "Validador_Node_01"); err != nil {
		t.Fatalf("AddBlock: %v", err)
	}
 
	// MAC correto (e com formato diferente) -> válido.
	if r := bc.ValidateDeviceMAC("172.25.10.52", "d0-94-66-e2-8e-93"); !r.IsValid {
		t.Errorf("MAC correto deveria validar; msg=%q", r.Message)
	}
 
	// MAC divergente -> spoofing detectado.
	if r := bc.ValidateDeviceMAC("172.25.10.52", "08:00:27:d1:f8:5d"); r.IsValid {
		t.Error("MAC divergente não deveria validar (ARP spoofing)")
	}
 
	// IP desconhecido -> não registrado.
	if r := bc.ValidateDeviceMAC("192.168.1.1", "aa:bb:cc:dd:ee:ff"); r.IsValid {
		t.Error("IP não registrado não deveria validar")
	}
}
 
// TestGetChainGenesisSafe garante que percorrer a cadeia até o gênese
func TestGetChainGenesisSafe(t *testing.T) {
	bc := newTestChain(t)
	dev := DeviceIdentity{ID: "SENSOR", IP: "172.25.10.52", MAC: "D0:94:66:E2:8E:93"}
	if err := bc.AddBlock(dev, "Validador_Node_01"); err != nil {
		t.Fatalf("AddBlock: %v", err)
	}
 
	chain := bc.GetChain() // não deve dar panic
	if len(chain) != 2 {
		t.Fatalf("esperava 2 blocos na cadeia, obtive %d", len(chain))
	}
 
	genesis := chain[len(chain)-1] // mais antigo = gênese
	if genesis.PrevHash != "" {
		t.Errorf("PrevHash do gênese deveria ser vazio, obtive %q", genesis.PrevHash)
	}
	if genesis.Validator != "System" {
		t.Errorf("validador do gênese deveria ser System, obtive %q", genesis.Validator)
	}
}