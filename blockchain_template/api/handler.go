package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"dmodbus/arp"
	"dmodbus/internal/blockchain"
	"dmodbus/network"
)

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		http.Error(w, fmt.Sprintf("Método não permitido, use %s", method), http.StatusMethodNotAllowed)
		return false
	}
	return true
}

// --- Handlers de consulta ---

// GET /query?ip=<ip> — dispositivo registrado para o IP.
func (s *Server) queryHandler(w http.ResponseWriter, r *http.Request) {
	ip := r.URL.Query().Get("ip")
	if ip == "" {
		http.Error(w, "Parâmetro 'ip' é obrigatório", http.StatusBadRequest)
		return
	}
	device, found := s.bc.FindDeviceByIP(ip)
	if !found {
		http.Error(w, "Dispositivo não encontrado na blockchain", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, device)
}

// GET /validate?ip=<ip> — valida o MAC atual do IP contra a ledger.
func (s *Server) validateHandler(w http.ResponseWriter, r *http.Request) {
	ip := r.URL.Query().Get("ip")
	if ip == "" {
		http.Error(w, "Parâmetro 'ip' é obrigatório", http.StatusBadRequest)
		return
	}

	entries, err := arp.ReadARPTable()
	if err != nil {
		http.Error(w, fmt.Sprintf("Erro ao ler tabela ARP: %v", err), http.StatusInternalServerError)
		return
	}

	var currentMAC string
	for _, e := range entries {
		if e.IP == ip {
			currentMAC = e.MAC
			break
		}
	}
	if currentMAC == "" {
		http.Error(w, "IP não encontrado na tabela ARP", http.StatusNotFound)
		return
	}

	result := s.bc.ValidateDeviceMAC(ip, currentMAC)
	status := http.StatusOK
	if !result.IsValid {
		status = http.StatusForbidden
	}
	writeJSON(w, status, result)
}

// GET /list — todos os dispositivos registrados.
func (s *Server) listHandler(w http.ResponseWriter, r *http.Request) {
	devices := s.bc.GetAllDevices()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total":   len(devices),
		"devices": devices,
	})
}

// GET /chain — toda a cadeia de blocos.
func (s *Server) chainHandler(w http.ResponseWriter, r *http.Request) {
	blocks := s.bc.GetChain()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total_blocks": len(blocks),
		"chain":        blocks,
	})
}

// GET /status — estado do nó atual.
func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"node_id":      s.nodeID,
		"address":      s.address,
		"block_count":  s.bc.GetBlockCount(),
		"device_count": len(s.bc.GetAllDevices()),
		"peer_count":   len(s.net.GetPeers()),
	})
}

// GET /integrity — verifica a integridade de toda a cadeia.
func (s *Server) integrityHandler(w http.ResponseWriter, r *http.Request) {

	valid, errs := s.bc.VerifyChainIntegrity()
	
	status := http.StatusOK
	if !valid {
		status = http.StatusConflict
	}
	writeJSON(w, status, map[string]interface{}{
		"valid":  valid,
		"errors": errs,
	})
}

// --- Handlers de sincronização ---

// POST /sync — lê o ARP local e sincroniza com a ledger.
func (s *Server) syncHandler(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	err := s.bc.SyncARPToBlockchain(s.nodeID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Erro na sincronização: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Sincronização ARP → Blockchain concluída",
	})
}

// --- Handlers P2P ---

// POST /receive-block — recebe um bloco propagado por outro nó.
func (s *Server) receiveBlockHandler(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Erro ao ler corpo da requisição", http.StatusBadRequest)
		return
	}

	var broadcast network.BlockBroadcast
	if err := json.Unmarshal(body, &broadcast); err != nil {
		http.Error(w, "Erro ao decodificar bloco", http.StatusBadRequest)
		return
	}

	log.Printf("Bloco recebido de %s", broadcast.Sender)

	if err := s.bc.AddBlockFromPeer(broadcast.Block); err != nil {
		log.Printf("Erro ao adicionar bloco recebido: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "block_received"})
}

// GET /peers — peers conhecidos por este nó.
func (s *Server) peersHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"node_id": s.nodeID,
		"peers":   s.net.GetPeers(),
	})
}

// GET /internal-query?ip=<ip> — usado por outros nós para consultar o MAC.
func (s *Server) internalQueryHandler(w http.ResponseWriter, r *http.Request) {
	ip := r.URL.Query().Get("ip")
	if ip == "" {
		http.Error(w, "Parâmetro 'ip' é obrigatório", http.StatusBadRequest)
		return
	}
	device, found := s.bc.FindDeviceByIP(ip)
	if !found {
		http.Error(w, "Dispositivo não encontrado", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, device)
}

// GET /distributed-query?ip=<ip> — consulta o MAC com consenso por maioria.
func (s *Server) distributedQueryHandler(w http.ResponseWriter, r *http.Request) {
	ip := r.URL.Query().Get("ip")
	if ip == "" {
		http.Error(w, "Parâmetro 'ip' é obrigatório", http.StatusBadRequest)
		return
	}

	log.Printf("Consulta distribuída para IP: %s", ip)

	peers := s.net.GetPeers()
	resultsChannel := make(chan string, len(peers))
	var wg sync.WaitGroup

	// Voto local.
	votes := make(map[string]int)
	if device, found := s.bc.FindDeviceByIP(ip); found {
		votes[device.MAC] = 1
		log.Printf("  - Voto local: %s", device.MAC)
	}

	// Votos dos peers em paralelo.
	wg.Add(len(peers))
	for _, peer := range peers {
		go func(p network.Peer) {
			defer wg.Done()
			mac, err := s.net.QueryPeerMAC(p.Address, ip)
			if err != nil {
				log.Printf("  - Peer %s: %v", p.NodeID, err)
				return
			}
			if mac != "" { // MAC vazio nunca vira voto
				resultsChannel <- mac
			}
		}(*peer)
	}

	go func() {
		wg.Wait()
		close(resultsChannel)
	}()

	for mac := range resultsChannel {
		votes[mac]++
		log.Printf("  - Voto de peer: %s", mac)
	}

	// Consenso por maioria (Eq. 1 do artigo, centralizada em blockchain.Quorum).
	totalNodes := len(peers) + 1
	threshold := blockchain.Quorum(totalNodes)

	var consensusMAC string
	maxVotes := 0
	for mac, count := range votes {
		if count > maxVotes {
			maxVotes = count
			consensusMAC = mac
		}
	}

	log.Printf("Resultado: %d/%d votos. Quórum: %d", maxVotes, totalNodes, threshold)

	if maxVotes >= threshold && consensusMAC != "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"consensus_reached": true,
			"ip":                ip,
			"mac":               consensusMAC,
			"votes":             maxVotes,
			"total_nodes":       totalNodes,
			"timestamp":         time.Now().Unix(),
		})
		return
	}

	writeJSON(w, http.StatusConflict, map[string]interface{}{
		"consensus_reached":  false,
		"ip":                 ip,
		"message":            "Consenso majoritário não atingido entre os nós.",
		"votes_distribution": votes,
		"total_nodes":        totalNodes,
	})
}