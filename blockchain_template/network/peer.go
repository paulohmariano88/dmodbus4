package network
 
import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
	"dmodbus/internal/blockchain"
)
 
// peerHTTPTimeout limita cada requisição a um peer. Sem ele, um peer travado
const peerHTTPTimeout = 2 * time.Second
 
// Peer descreve um nó remoto conhecido.
type Peer struct {
	Address  string `json:"address"`  // ex.: "http://localhost:8081"
	NodeID   string `json:"node_id"`  // ex.: "Validador_Node_02"
	IsActive bool   `json:"is_active"`
	LastSeen int64  `json:"last_seen"`
}
 
// BlockBroadcast é o envelope enviado a um peer ao propagar um bloco.
type BlockBroadcast struct {
	Block     *blockchain.Block `json:"block"`
	Sender    string            `json:"sender"`
	Timestamp int64             `json:"timestamp"`
}
 
// PeerNetwork mantém o conjunto de peers conhecidos e o cliente HTTP usado para falar com eles.
type PeerNetwork struct {
	peers  map[string]*Peer
	nodeID string
	client *http.Client
	mu     sync.RWMutex
}
 
// NewPeerNetwork cria uma rede vazia para o nó nodeID.
func NewPeerNetwork(nodeID string) *PeerNetwork {
	return &PeerNetwork{
		peers:  make(map[string]*Peer),
		nodeID: nodeID,
		client: &http.Client{Timeout: peerHTTPTimeout},
	}
}
 
// NodeID devolve o identificador deste nó.
func (pn *PeerNetwork) NodeID() string {
	return pn.nodeID
}
 
// AddPeer registra (ou atualiza) um peer pelo endereço.
func (pn *PeerNetwork) AddPeer(address, nodeID string) {
	pn.mu.Lock()
	defer pn.mu.Unlock()
 
	pn.peers[address] = &Peer{
		Address:  address,
		NodeID:   nodeID,
		IsActive: true,
		LastSeen: time.Now().Unix(),
	}
	log.Printf("Peer adicionado: %s (%s)", nodeID, address)
}
 
// GetPeers devolve uma cópia da lista de peers conhecidos.
func (pn *PeerNetwork) GetPeers() []*Peer {
	pn.mu.RLock()
	defer pn.mu.RUnlock()
 
	peers := make([]*Peer, 0, len(pn.peers))
	for _, p := range pn.peers {
		peers = append(peers, p)
	}
	return peers
}
 
// BroadcastBlock propaga um bloco para todos os peers em paralelo.
func (pn *PeerNetwork) BroadcastBlock(block *blockchain.Block) {
	payload, err := json.Marshal(BlockBroadcast{
		Block:     block,
		Sender:    pn.nodeID,
		Timestamp: time.Now().Unix(),
	})
	if err != nil {
		log.Printf("erro ao serializar bloco para broadcast: %v", err)
		return
	}
 
	for _, peer := range pn.GetPeers() {
		go func(addr string) {
			resp, err := pn.client.Post(addr+"/receive-block", "application/json", bytes.NewReader(payload))
			if err != nil {
				log.Printf("erro ao enviar bloco para %s: %v", addr, err)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				log.Printf("Bloco enviado para %s", addr)
			}
		}(peer.Address)
	}
}
 
// DiscoverPeers pergunta a cada peer conhecido quais peers ele conhece e
// incorpora os novos (exceto o próprio nó).
func (pn *PeerNetwork) DiscoverPeers(myAddress string) {
	log.Println("Iniciando descoberta de peers...")
 
	for _, peer := range pn.GetPeers() {
		go func(addr string) {
			resp, err := pn.client.Get(addr + "/peers")
			if err != nil {
				return
			}
			defer resp.Body.Close()
 
			var result struct {
				Peers []Peer `json:"peers"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return
			}
			for _, discovered := range result.Peers {
				if discovered.Address != myAddress {
					pn.AddPeer(discovered.Address, discovered.NodeID)
				}
			}
		}(peer.Address)
	}
}
 
// QueryPeerMAC consulta o MAC confiável que um peer tem registrado para um IP
func (pn *PeerNetwork) QueryPeerMAC(peerAddr, ip string) (string, error) {
	url := fmt.Sprintf("%s/internal-query?ip=%s", peerAddr, ip)
	resp, err := pn.client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
 
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("peer %s respondeu status %d", peerAddr, resp.StatusCode)
	}
 
	var device blockchain.DeviceIdentity
	if err := json.NewDecoder(resp.Body).Decode(&device); err != nil {
		return "", fmt.Errorf("erro ao decodificar resposta de %s: %w", peerAddr, err)
	}
	return device.MAC, nil
}