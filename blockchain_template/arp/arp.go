package arp
 
import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)
 
// ARPEntry representa um par IP -> MAC observado na tabela ARP local.
type ARPEntry struct {
	IP  string
	MAC string
}
 
// arpLineRE captura o IP e o MAC de uma linha de saída do comando `arp -a`,
// tolerando os formatos comuns de Linux e Windows (separador ":" ou "-").
var arpLineRE = regexp.MustCompile(
	`(?:\(|\s)([\d\.]+)(?:\)|\s+)(?:at\s+|ether\s+)` +
		`([\da-fA-F]{1,2}[:-][\da-fA-F]{1,2}[:-][\da-fA-F]{1,2}` +
		`[:-][\da-fA-F]{1,2}[:-][\da-fA-F]{1,2}[:-][\da-fA-F]{1,2})`,
)
 
// NormalizeMAC padroniza um endereço MAC para MAIÚSCULAS com separador ":".
// Ex.: "d0-94-66-e2-8e-93" -> "D0:94:66:E2:8E:93".
func NormalizeMAC(mac string) string {
	mac = strings.ToUpper(mac)
	mac = strings.ReplaceAll(mac, "-", ":")
	return mac
}
 
// ReadARPTable executa `arp -a` e devolve as entradas válidas encontradas.
func ReadARPTable() ([]ARPEntry, error) {
	output, err := exec.Command("arp", "-a").Output()
	if err != nil {
		return nil, fmt.Errorf("erro ao executar comando arp: %w", err)
	}
	return parseARPOutput(string(output)), nil
}
 
// parseARPOutput extrai as entradas da saída textual do comando arp.
// Endereços inválidos (0.0.0.0) e link-local (169.254.x.x) são descartados.
func parseARPOutput(output string) []ARPEntry {
	entries := make([]ARPEntry, 0)
	for _, line := range strings.Split(output, "\n") {
		m := arpLineRE.FindStringSubmatch(line)
		if len(m) != 3 {
			continue
		}
		ip, mac := m[1], NormalizeMAC(m[2])
		if ip == "0.0.0.0" || strings.HasPrefix(ip, "169.254") {
			continue
		}
		entries = append(entries, ARPEntry{IP: ip, MAC: mac})
	}
	return entries
}