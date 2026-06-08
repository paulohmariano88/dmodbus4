package blockchain

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/syndtr/goleveldb/leveldb"

	"dmodbus/arp"
)

// lastHashKey é a chave fixa que aponta para o hash do último bloco da cadeia.
const lastHashKey = "l"

var ipRegex = regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`)

type Broadcaster interface {
	BroadcastBlock(b *Block)
}


type ValidationResult struct {
	IsValid     bool   `json:"is_valid"`
	IP          string `json:"ip"`
	ExpectedMAC string `json:"expected_mac"`
	DetectedMAC string `json:"detected_mac"`
	Message     string `json:"message"`
}



type BlockView struct {
	Timestamp string         `json:"timestamp"`
	Validator string         `json:"validator"`
	Hash      string         `json:"hash"`
	PrevHash  string         `json:"prev_hash"`
	Device    DeviceIdentity `json:"device"`
}

// Blockchain é a ledger permissionada de um nó.
type Blockchain struct {
	db          *leveldb.DB
	lastHash    []byte
	validators  map[string]bool
	broadcaster Broadcaster
	mu          sync.RWMutex
}

// Quorum implementa o limiar de consenso:ou seja, a maioria estrita de N nós validadores.
func Quorum(n int) int {
	return n/2 + 1
}


func NewBlockchain(dbPath string, authorizedValidators []string, broadcaster Broadcaster) (*Blockchain, error) {
	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir LevelDB em %q: %w", dbPath, err)
	}

	lastHash, err := db.Get([]byte(lastHashKey), nil)
	switch {
	case errors.Is(err, leveldb.ErrNotFound):
		log.Println("Nenhuma ledger encontrada em disco. Criando bloco gênese...")
		genesis := NewBlock(
			DeviceIdentity{
				ID:        "GENESIS",
				IP:        "0.0.0.0",
				MAC:       "00:00:00:00:00:00",
				Timestamp: time.Now().Unix(),
			},
			[]byte{},
			"System",
		)
		batch := new(leveldb.Batch)
		batch.Put(genesis.Hash, genesis.Serialize())
		batch.Put([]byte(lastHashKey), genesis.Hash)
		if err := db.Write(batch, nil); err != nil {
			db.Close()
			return nil, fmt.Errorf("erro ao gravar bloco gênese: %w", err)
		}
		lastHash = genesis.Hash
	case err != nil:
		db.Close()
		return nil, fmt.Errorf("erro ao ler último hash: %w", err)
	default:
		log.Println("Ledger existente encontrada em disco. Carregando...")
	}

	validators := make(map[string]bool, len(authorizedValidators))
	for _, v := range authorizedValidators {
		validators[v] = true
	}

	return &Blockchain{
		db:          db,
		lastHash:    lastHash,
		validators:  validators,
		broadcaster: broadcaster,
	}, nil
}

// Close libera o handle do banco. Deve ser chamado pelo entrypoint via defer.
func (bc *Blockchain) Close() error {
	return bc.db.Close()
}

// AddBlock cria e persiste um novo bloco a partir de uma identidade local,
// depois o propaga para os peers. Apenas validadores autorizados podem
// escrever (regra de autoridade do PoA).
func (bc *Blockchain) AddBlock(data DeviceIdentity, validatorID string) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if !bc.validators[validatorID] {
		return fmt.Errorf("nó %q não tem autoridade para adicionar blocos", validatorID)
	}

	data.MAC = arp.NormalizeMAC(data.MAC)
	newBlock := NewBlock(data, bc.lastHash, validatorID)

	batch := new(leveldb.Batch)
	batch.Put(newBlock.Hash, newBlock.Serialize())
	batch.Put([]byte(data.IP), []byte(data.MAC)) // índice IP -> MAC
	batch.Put([]byte(lastHashKey), newBlock.Hash)

	if err := bc.db.Write(batch, nil); err != nil {
		return fmt.Errorf("erro ao gravar bloco: %w", err)
	}

	bc.lastHash = newBlock.Hash
	log.Printf("Bloco %x... adicionado: %s (%s -> %s) por %s",
		newBlock.Hash[:4], data.ID, data.IP, data.MAC, validatorID)

	if bc.broadcaster != nil {
		bc.broadcaster.BroadcastBlock(newBlock)
	}
	return nil
}

// AddBlockFromPeer persiste um bloco recebido de outro nó, validando autoria
// e ignorando duplicatas.
func (bc *Blockchain) AddBlockFromPeer(block *Block) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if _, err := bc.db.Get(block.Hash, nil); err == nil {
		return nil // já temos este bloco
	}

	if !bc.validators[block.Validator] {
		return fmt.Errorf("validador não autorizado: %q", block.Validator)
	}

	var identity DeviceIdentity
	if err := json.Unmarshal(block.Data, &identity); err != nil {
		return fmt.Errorf("erro ao decodificar identidade do bloco: %w", err)
	}

	batch := new(leveldb.Batch)
	batch.Put(block.Hash, block.Serialize())
	batch.Put([]byte(identity.IP), []byte(identity.MAC))
	batch.Put([]byte(lastHashKey), block.Hash)

	if err := bc.db.Write(batch, nil); err != nil {
		return fmt.Errorf("erro ao gravar bloco de peer: %w", err)
	}

	bc.lastHash = block.Hash
	log.Printf("Bloco recebido de peer: %x... (%s)", block.Hash[:4], identity.ID)
	return nil
}

// FindDeviceByIP busca o MAC confiável associado a um IP no índice da ledger.
func (bc *Blockchain) FindDeviceByIP(ip string) (*DeviceIdentity, bool) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	macBytes, err := bc.db.Get([]byte(ip), nil)
	if err != nil {
		return nil, false
	}
	return &DeviceIdentity{IP: ip, MAC: string(macBytes)}, true
}

// GetAllDevices devolve todos os pares IP -> MAC indexados na ledger.
func (bc *Blockchain) GetAllDevices() []DeviceIdentity {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	devices := make([]DeviceIdentity, 0)
	iter := bc.db.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		key := string(iter.Key())
		if ipRegex.MatchString(key) {
			devices = append(devices, DeviceIdentity{
				IP:  key,
				MAC: string(iter.Value()),
			})
		}
	}
	return devices
}

// GetBlockCount percorre a cadeia a partir do último hash e conta os blocos.
func (bc *Blockchain) GetBlockCount() int {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	count := 0
	currentHash := bc.lastHash
	for {
		data, err := bc.db.Get(currentHash, nil)
		if err != nil {
			break
		}
		block := DeserializeBlock(data)
		count++
		if len(block.PrevBlockHash) == 0 {
			break
		}
		currentHash = block.PrevBlockHash
	}
	return count
}

// GetChain devolve a cadeia inteira como projeções BlockView
func (bc *Blockchain) GetChain() []BlockView {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	views := make([]BlockView, 0)
	currentHash := bc.lastHash
	for {
		data, err := bc.db.Get(currentHash, nil)
		if err != nil {
			break
		}
		block := DeserializeBlock(data)

		var identity DeviceIdentity
		json.Unmarshal(block.Data, &identity)

		views = append(views, BlockView{
			Timestamp: time.Unix(block.Timestamp, 0).Format("2006-01-02 15:04:05"),
			Validator: block.Validator,
			Hash:      shortHex(block.Hash),
			PrevHash:  shortHex(block.PrevBlockHash),
			Device:    identity,
		})

		if len(block.PrevBlockHash) == 0 {
			break
		}
		currentHash = block.PrevBlockHash
	}
	return views
}

// SyncARPToBlockchain lê a tabela ARP e registra na ledger os dispositivos
// ainda não conhecidos. Para IPs já registrados com MAC divergente, emite um
// alerta (possível ARP spoofing) sem sobrescrever o registro confiável.
func (bc *Blockchain) SyncARPToBlockchain(validatorID string) error {
	entries, err := arp.ReadARPTable()
	if err != nil {
		return err
	}
	log.Printf("Tabela ARP: %d dispositivos encontrados", len(entries))

	registered := 0
	for _, entry := range entries {
		if existing, found := bc.FindDeviceByIP(entry.IP); found {
			if existing.MAC != entry.MAC {
				log.Printf("ALERTA: MAC divergente para %s | ledger=%s ARP=%s",
					entry.IP, existing.MAC, entry.MAC)
			}
			continue
		}

		device := DeviceIdentity{
			ID:        fmt.Sprintf("DEVICE_%s", strings.ReplaceAll(entry.IP, ".", "_")),
			IP:        entry.IP,
			MAC:       entry.MAC,
			Timestamp: time.Now().Unix(),
		}
		if err := bc.AddBlock(device, validatorID); err != nil {
			log.Printf("erro ao registrar %s: %v", entry.IP, err)
			continue
		}
		registered++
	}
	if registered > 0 {
		log.Printf("%d novos dispositivos registrados na ledger", registered)
	}
	return nil
}


func (bc *Blockchain) ValidateDeviceMAC(ip, detectedMAC string) ValidationResult {
	detected := arp.NormalizeMAC(detectedMAC)

	device, found := bc.FindDeviceByIP(ip)
	if !found {
		return ValidationResult{
			IsValid:     false,
			IP:          ip,
			DetectedMAC: detected,
			Message:     "Dispositivo não registrado na blockchain",
		}
	}

	if device.MAC == detected {
		return ValidationResult{
			IsValid:     true,
			IP:          ip,
			ExpectedMAC: device.MAC,
			DetectedMAC: detected,
			Message:     "Dispositivo autenticado com sucesso",
		}
	}
	return ValidationResult{
		IsValid:     false,
		IP:          ip,
		ExpectedMAC: device.MAC,
		DetectedMAC: detected,
		Message:     "ALERTA: Possível ARP Spoofing detectado!",
	}
}

// shortHex formata os primeiros bytes de um hash em hexadecimal, de forma
// segura quando o slice é menor que 8 bytes ou vazio (caso do gênese).
func shortHex(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	n := 8
	if len(b) < n {
		n = len(b)
	}
	return fmt.Sprintf("%x", b[:n])
}

// VerifyChainIntegrity percorre a cadeia a partir do último bloco até o gênese,
// garantindo que todos os blocos intermediários existam e estejam interligados.
func (bc *Blockchain) VerifyChainIntegrity() (bool, []string) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	var errs []string
	currentHash := bc.lastHash

	// Se por algum motivo não houver hash inicial
	if len(currentHash) == 0 {
		errs = append(errs, "Cadeia vazia ou hash inicial inválido")
		return false, errs
	}

	for {
		// Busca o bloco atual no LevelDB usando o hash
		data, err := bc.db.Get(currentHash, nil)
		if err != nil {
			errs = append(errs, fmt.Sprintf("Quebra na cadeia: Bloco com hash %x não encontrado no LevelDB", currentHash))
			return false, errs
		}

		// Deserializa para poder ler o ponteiro do bloco anterior
		block := DeserializeBlock(data)

		// Se o bloco anterior for vazio, significa que chegamos com sucesso ao bloco Gênese
		if len(block.PrevBlockHash) == 0 {
			break
		}

		// Avança para o hash do bloco anterior
		currentHash = block.PrevBlockHash
	}

	return len(errs) == 0, errs
}