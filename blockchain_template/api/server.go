// Package api expõe a ledger e a rede P2P do nó via HTTP.

package api
 
import (
	"log"
	"net/http"
 
	"dmodbus/internal/blockchain"
	"dmodbus/network"
)
 
// Server agrega as dependências necessárias aos handlers HTTP.
type Server struct {
	bc      *blockchain.Blockchain
	net     *network.PeerNetwork
	nodeID  string
	address string
}
 
// NewServer cria um Server com a ledger e a rede já inicializadas.
func NewServer(bc *blockchain.Blockchain, net *network.PeerNetwork, nodeID, address string) *Server {
	return &Server{
		bc:      bc,
		net:     net,
		nodeID:  nodeID,
		address: address,
	}
}
 
// Routes registra todas as rotas e devolve o multiplexer.
func (s *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()
 
	// Endpoints de consulta e validação.
	mux.HandleFunc("/query", s.queryHandler)
	mux.HandleFunc("/validate", s.validateHandler)
	mux.HandleFunc("/sync", s.syncHandler)
	mux.HandleFunc("/list", s.listHandler)
	mux.HandleFunc("/chain", s.chainHandler)
	mux.HandleFunc("/status", s.statusHandler)
 
	// Endpoints P2P.
	mux.HandleFunc("/receive-block", s.receiveBlockHandler)
	mux.HandleFunc("/peers", s.peersHandler)
	mux.HandleFunc("/internal-query", s.internalQueryHandler)
	mux.HandleFunc("/distributed-query", s.distributedQueryHandler)
 
	return mux
}
 
// ListenAndServe sobe o servidor HTTP na porta informada.
func (s *Server) ListenAndServe(port string) error {
	log.Printf("Servidor escutando em :%s", port)
	return http.ListenAndServe(":"+port, s.Routes())
}