package main
 
import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
 
	"dmodbus/api"
	"dmodbus/internal/blockchain"
	"dmodbus/network"
)
 
// Validadores autorizados na rede permissionada (PoA). 
var authorizedValidators = []string{
	"Validador_Node_01",
	"Validador_Node_02",
	"Validador_Node_03",
	"System_ARP_Scan",
}
 
func main() {
	port := getenv("PORT", "8080")
	nodeID := getenv("NODE_ID", "Validador_Node_01")
	address := fmt.Sprintf("http://localhost:%s", port)
 
	// Cada nó mantém sua própria cópia da ledger em disco.
	dbPath := fmt.Sprintf("dmodbus_ledger_%s", strings.ReplaceAll(nodeID, " ", "_"))
 
	log.Println(strings.Repeat("=", 70))
	log.Printf("DMODBUS P2P Node - %s", nodeID)
	log.Println(strings.Repeat("=", 70))
 
	// 1. Rede P2P primeiro.
	net := network.NewPeerNetwork(nodeID)
 
	// 2. Blockchain recebe a rede como Broadcaster (injeção de dependência).
	//    NewBlockchain agora devolve erro em vez de dar panic, para que o
	//    entrypoint controle o ciclo de vida do processo.
	bc, err := blockchain.NewBlockchain(dbPath, authorizedValidators, net)
	if err != nil {
		log.Fatalf("erro ao inicializar a blockchain: %v", err)
	}
	defer bc.Close()
 
	// Peers conhecidos via env: PEERS="http://localhost:8081, http://localhost:8082"
	if peers := os.Getenv("PEERS"); peers != "" {
		for _, addr := range strings.Split(peers, ",") {
			net.AddPeer(strings.TrimSpace(addr), "Unknown")
		}
	}
 
	// 3. Servidor HTTP com referências para ledger e rede.
	srv := api.NewServer(bc, net, nodeID, address)
 
	// Inicialização em background: sincroniza ARP -> ledger e descobre peers,
	// sem bloquear a subida do servidor.
	go func() {
		time.Sleep(2 * time.Second)
 
		log.Println("Sincronizando tabela ARP com a blockchain...")
		if err := bc.SyncARPToBlockchain(nodeID); err != nil {
			log.Printf("aviso: falha na sincronização ARP: %v", err)
		}
 
		net.DiscoverPeers(address)
 
		log.Println(strings.Repeat("=", 70))
		log.Printf("Nó pronto | blocos=%d dispositivos=%d peers=%d",
			bc.GetBlockCount(), len(bc.GetAllDevices()), len(net.GetPeers()))
		log.Println(strings.Repeat("=", 70))
	}()
 
	log.Fatal(srv.ListenAndServe(port))
}
 
// getenv devolve a variável de ambiente k ou o valor padrão def quando vazia.
func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}