import sys
import requests
import subprocess
import platform
from scapy.all import sniff, IP, TCP

# Configurações do nó da Blockchain DModbus
BLOCKCHAIN_NODE_IP = "localhost"  # Altere para o IP do nó da blockchain se não estiver rodando localmente
BLOCKCHAIN_PORT = "8080"

# --- Funções de consulta ---
def get_blockchain_mac(ip):
    """Consulta a API da blockchain para obter o MAC confiável."""
    url = f"http://{BLOCKCHAIN_NODE_IP}:{BLOCKCHAIN_PORT}/query?ip={ip}"
    try:
        response = requests.get(url, timeout=5)
        if response.status_code == 200:
            return response.json()['mac'].lower().replace(':', '-')
    except requests.exceptions.RequestException:
        # Silencioso para não poluir a saída do sniffer em tempo real
        pass
    return None

def get_local_arp_mac(ip):
    """Executa 'arp -a' ou 'arp -n' dependendo do S.O. e busca o MAC para um IP específico."""
    os_type = platform.system().lower()
    command = ["arp", "-a"] if os_type == "windows" else ["arp", "-n"]
    try:
        result = subprocess.check_output(command).decode('latin-1')
        for line in result.split('\n'):
            if ip in line:
                parts = line.split()
                for part in parts:
                    if part.count('-') == 5 or part.count(':') == 5:
                        return part.lower().replace(':', '-')
    except Exception:
        return None
    return None

# --- A LÓGICA DE TEMPO REAL ---
def packet_callback(packet):
    """Executada para cada pacote Modbus/TCP capturado na rede."""
    if not packet.haslayer(IP) or not packet.haslayer(TCP):
        return

    # Extração dos IPs de origem e destino
    ip_src = packet[IP].src
    ip_dst = packet[IP].dst
    
    print(f"\n--- Pacote Modbus Detectado: {ip_src} -> {ip_dst} ---")
    
    # Validação da identidade do remetente (ip_src) contra a Ledger PoA
    blockchain_mac = get_blockchain_mac(ip_src)
    local_mac = get_local_arp_mac(ip_src)

    print(f"  Verificando IP de origem: {ip_src}")
    print(f"  -> MAC Confiável (Blockchain): {blockchain_mac}")
    print(f"  -> MAC na Rede Local (Tabela ARP): {local_mac}")

    if blockchain_mac and local_mac:
        if blockchain_mac == local_mac:
            print("  [STATUS] OK: Identidade do remetente é válida.")
        else:
            # Output de alerta em caso de divergência (Sinalização de ARP Spoofing / MitM)
            print("  \033[91m[ALERTA!] ATAQUE DETECTADO! O MAC do remetente na rede local não confere com a blockchain!\033[0m")
    elif not blockchain_mac:
        print(f"  [AVISO] O remetente {ip_src} não está registrado na blockchain.")
    else:
        print(f"  [AVISO] Não foi possível obter o MAC da rede local para o remetente {ip_src}.")


def main():
    """Função principal que gerencia os parâmetros de interface e inicia o sniffer."""
    # Interface padrão caso o usuário não passe nenhum argumento
    iface_name = "Ethernet"

    # Se um argumento foi fornecido via linha de comando, adota-o como interface alvo
    # Exemplo: python gateway.py eth0
    if len(sys.argv) > 1:
        iface_name = sys.argv[1]

    print("--- DMODBUS Gateway em Tempo Real Iniciado ---")
    print(f"Interface ativa para monitoramento: '{iface_name}'")
    print("Escutando por tráfego Modbus (porta 502)... Pressione CTRL+C para parar.")
    
    # Inicialização do módulo de captura da Scapy (Non-invasive Sniffing)
    try:
        sniff(iface=iface_name, filter="tcp and port 502", prn=packet_callback, store=0)
    except Exception as e:
        print(f"\n[ERRO] Falha ao iniciar o sniffer na interface '{iface_name}'.")
        print("Dicas de Resolução:")
        print(" 1. Verifique se executou o script com privilégios de Administrador/Sudo.")
        print(" 2. Certifique-se de que o nome da interface está correto para o seu sistema.")
        print(f" Erro detalhado: {e}")

if __name__ == "__main__":
    main()