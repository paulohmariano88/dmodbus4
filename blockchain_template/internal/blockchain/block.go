// Package blockchain implementa a ledger permissionada do DModbus: blocos
// encadeados por hash SHA-256 que notarizam a identidade (ID/IP/MAC) de cada
// dispositivo da rede OT.
//
// Este arquivo (block.go) concentra a unidade de dados — o Block — e o registro
// de identidade que ele carrega. A lógica da cadeia (Blockchain) fica em
// blockchain.go.
package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/json"
	"log"
	"strconv"
	"time"

	"dmodbus/arp"
)

// DeviceIdentity é o registro de identidade notarizado na ledger.
// Os campos correspondem aos identificadores descritos no artigo: o ID
// (Unit Identifier do Modbus), o IP canônico e o MAC físico do dispositivo.
type DeviceIdentity struct {
	ID        string `json:"id"`
	IP        string `json:"ip"`
	MAC       string `json:"mac"`
	Timestamp int64  `json:"timestamp"`
}

// Block é a unidade encadeada da ledger. Data carrega um DeviceIdentity
// serializado em JSON; Validator identifica o nó PoA que confirmou o bloco.
type Block struct {
	Timestamp     int64
	Data          []byte
	PrevBlockHash []byte
	Hash          []byte
	Validator     string
}

// Serialize codifica o bloco com gob para persistência no LevelDB.
func (b *Block) Serialize() []byte {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(b); err != nil {
		log.Panicf("erro ao serializar bloco: %v", err)
	}
	return buf.Bytes()
}

// DeserializeBlock reconstrói um bloco a partir dos bytes persistidos.
// Uma falha aqui indica ledger corrompida (invariante interna), por isso
// interrompe a execução.
func DeserializeBlock(d []byte) *Block {
	var block Block
	if err := gob.NewDecoder(bytes.NewReader(d)).Decode(&block); err != nil {
		log.Panicf("erro ao desserializar bloco: %v", err)
	}
	return &block
}

// SetHash calcula o SHA-256 sobre o cabeçalho do bloco (prev hash + dados +
// timestamp + validador) e o grava em b.Hash.
func (b *Block) SetHash() {
	timestamp := []byte(strconv.FormatInt(b.Timestamp, 10))
	validator := []byte(b.Validator)
	headers := bytes.Join(
		[][]byte{b.PrevBlockHash, b.Data, timestamp, validator},
		[]byte{},
	)
	hash := sha256.Sum256(headers)
	b.Hash = hash[:]
}

// NewBlock cria um bloco já com hash calculado. O MAC do dispositivo é
// normalizado antes da serialização, garantindo que o hash seja estável
// independentemente do formato de origem (Linux/Windows).
func NewBlock(data DeviceIdentity, prevBlockHash []byte, validator string) *Block {
	data.MAC = arp.NormalizeMAC(data.MAC)

	dataBytes, err := json.Marshal(data)
	if err != nil {
		log.Panicf("erro ao serializar identidade do dispositivo: %v", err)
	}

	block := &Block{
		Timestamp:     time.Now().Unix(),
		Data:          dataBytes,
		PrevBlockHash: prevBlockHash,
		Hash:          []byte{},
		Validator:     validator,
	}
	block.SetHash()
	return block
}