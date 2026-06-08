"""
Cliente Modbus TCP para Consumir Dados de Bateria de Trem
Conecta ao servidor Modbus e lê os dados de aferição da bateria
"""

from pymodbus.client import ModbusTcpClient
from pymodbus.exceptions import ModbusException
import time
import logging
from datetime import datetime

# Configurar logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


class BatteryModbusClient:
    """Cliente Modbus para consumir dados da bateria"""
    
    def __init__(self, host, port=502, unit_id=1):
        """
        Inicializa o cliente Modbus TCP
        
        Args:
            host: Endereço IP do servidor Modbus
            port: Porta TCP (padrão 5020)
            unit_id: ID da unidade Modbus (slave ID)
        """
        self.host = host
        self.port = port
        self.unit_id = unit_id
        self.client = None
        
        # Mapeamento de registros (conforme servidor)
        self.REGISTERS = {
            'voltage': (0, 2),      # Endereço 0, 2 registros
            'current': (2, 2),      # Endereço 2, 2 registros
            'temperature': (4, 1),  # Endereço 4, 1 registro
            'soc': (6, 1),         # Endereço 6, 1 registro
            'soh': (7, 1),         # Endereço 7, 1 registro
            'cycles': (8, 2),      # Endereço 8, 2 registros
            'status': (10, 1)      # Endereço 10, 1 registro
        }
        
        # Mapeamento de status
        self.STATUS_MAP = {
            0: 'ERRO',
            1: 'NORMAL',
            2: 'CARREGANDO',
            3: 'DESCARREGANDO'
        }
    
    def connect(self):
        """Estabelece conexão com o servidor Modbus"""
        try:
            self.client = ModbusTcpClient(self.host, port=self.port)
            connected = self.client.connect()
            if connected:
                logger.info(f"✓ Conectado ao servidor {self.host}:{self.port}")
                return True
            else:
                logger.error(f"✗ Falha ao conectar em {self.host}:{self.port}")
                return False
        except Exception as e:
            logger.error(f"✗ Erro ao conectar: {e}")
            return False
    
    def disconnect(self):
        """Fecha a conexão com o servidor Modbus"""
        if self.client:
            self.client.close()
            logger.info("Conexão fechada")
    
    def is_connected(self):
        """Verifica se está conectado"""
        return self.client and self.client.is_socket_open()
    
    def read_registers(self, address, count):
        """
        Lê registros holding do servidor
        
        Args:
            address: Endereço inicial
            count: Número de registros
            
        Returns:
            Lista de valores ou None em caso de erro
        """
        try:
            if not self.is_connected():
                logger.warning("Cliente não está conectado. Tentando reconectar...")
                if not self.connect():
                    return None
            
            response = self.client.read_holding_registers(
                address=address,
                count=count,
                slave=self.unit_id
            )
            
            if response.isError():
                logger.error(f"Erro ao ler registros {address}: {response}")
                return None
            
            return response.registers
            
        except ModbusException as e:
            logger.error(f"Exceção Modbus: {e}")
            return None
        except Exception as e:
            logger.error(f"Erro inesperado: {e}")
            return None
    
    def registers_to_float(self, registers):
        """Converte 2 registros em float (reverso do servidor)"""
        if not registers or len(registers) < 2:
            return None
        high = registers[0]
        low = registers[1]
        int_value = (high << 16) | low
        # Trata como signed
        if int_value > 0x7FFFFFFF:
            int_value -= 0x100000000
        return int_value / 100.0
    
    def registers_to_int32(self, registers):
        """Converte 2 registros em inteiro de 32 bits"""
        if not registers or len(registers) < 2:
            return None
        high = registers[0]
        low = registers[1]
        return (high << 16) | low
    
    def read_voltage(self):
        """Lê a tensão da bateria (V)"""
        addr, count = self.REGISTERS['voltage']
        regs = self.read_registers(addr, count)
        return self.registers_to_float(regs) if regs else None
    
    def read_current(self):
        """Lê a corrente da bateria (A)"""
        addr, count = self.REGISTERS['current']
        regs = self.read_registers(addr, count)
        return self.registers_to_float(regs) if regs else None
    
    def read_temperature(self):
        """Lê a temperatura da bateria (°C)"""
        addr, count = self.REGISTERS['temperature']
        regs = self.read_registers(addr, count)
        return regs[0] / 10.0 if regs else None
    
    def read_soc(self):
        """Lê o estado de carga (%)"""
        addr, count = self.REGISTERS['soc']
        regs = self.read_registers(addr, count)
        return regs[0] / 10.0 if regs else None
    
    def read_soh(self):
        """Lê o estado de saúde (%)"""
        addr, count = self.REGISTERS['soh']
        regs = self.read_registers(addr, count)
        return regs[0] / 10.0 if regs else None
    
    def read_cycles(self):
        """Lê o número de ciclos"""
        addr, count = self.REGISTERS['cycles']
        regs = self.read_registers(addr, count)
        return self.registers_to_int32(regs) if regs else None
    
    def read_status(self):
        """Lê o status da bateria"""
        addr, count = self.REGISTERS['status']
        regs = self.read_registers(addr, count)
        if regs:
            status_code = regs[0]
            return {
                'code': status_code,
                'description': self.STATUS_MAP.get(status_code, 'DESCONHECIDO')
            }
        return None
    
    def read_all_data(self):
        """Lê todos os dados da bateria"""
        timestamp = datetime.now()
        
        data = {
            'timestamp': timestamp.strftime('%Y-%m-%d %H:%M:%S'),
            'voltage': self.read_voltage(),
            'current': self.read_current(),
            'temperature': self.read_temperature(),
            'soc': self.read_soc(),
            'soh': self.read_soh(),
            'cycles': self.read_cycles(),
            'status': self.read_status()
        }
        
        # Calcula potência se tensão e corrente estiverem disponíveis
        if data['voltage'] is not None and data['current'] is not None:
            data['power'] = data['voltage'] * data['current']
        else:
            data['power'] = None
        
        return data
    
    def print_data(self, data):
        """Formata e exibe os dados de forma legível"""
        print("\n" + "="*70)
        print(f"{'DADOS DA BATERIA':^70}")
        print("="*70)
        print(f"Timestamp: {data['timestamp']}")
        print("-"*70)
        
        # Tensão
        if data['voltage'] is not None:
            print(f"⚡ Tensão:       {data['voltage']:>8.2f} V")
        else:
            print(f"⚡ Tensão:       {'N/A':>8}")
        
        # Corrente
        if data['current'] is not None:
            print(f"🔌 Corrente:     {data['current']:>8.2f} A")
        else:
            print(f"🔌 Corrente:     {'N/A':>8}")
        
        # Potência
        if data['power'] is not None:
            print(f"⚙️  Potência:     {data['power']:>8.2f} W")
        else:
            print(f"⚙️  Potência:     {'N/A':>8}")
        
        # Temperatura
        if data['temperature'] is not None:
            print(f"🌡️  Temperatura:  {data['temperature']:>8.1f} °C")
        else:
            print(f"🌡️  Temperatura:  {'N/A':>8}")
        
        # Estado de Carga
        if data['soc'] is not None:
            bar_length = int(data['soc'] / 5)  # Barra de 0-20 chars
            bar = '█' * bar_length + '░' * (20 - bar_length)
            print(f"🔋 SoC:          {data['soc']:>7.1f} % [{bar}]")
        else:
            print(f"🔋 SoC:          {'N/A':>8}")
        
        # Estado de Saúde
        if data['soh'] is not None:
            print(f"💚 SoH:          {data['soh']:>8.1f} %")
        else:
            print(f"💚 SoH:          {'N/A':>8}")
        
        # Ciclos
        if data['cycles'] is not None:
            print(f"🔄 Ciclos:       {data['cycles']:>8}")
        else:
            print(f"🔄 Ciclos:       {'N/A':>8}")
        
        # Status
        if data['status']:
            status_emoji = {
                'ERRO': '',
                'NORMAL': '',
                'CARREGANDO': '⬆',
                'DESCARREGANDO': '⬇'
            }
            emoji = status_emoji.get(data['status']['description'], '❓')
            print(f"{emoji} Status:       {data['status']['description']}")
        else:
            print(f" Status:       N/A")
        
        print("="*70)
    
    def monitor_continuous(self, interval=2):
        """
        Monitora continuamente a bateria
        
        Args:
            interval: Intervalo entre leituras em segundos
        """
        logger.info(f"Iniciando monitoramento contínuo (intervalo: {interval}s)")
        print("\n💡 Pressione Ctrl+C para parar o monitoramento\n")
        
        try:
            while True:
                data = self.read_all_data()
                self.print_data(data)
                time.sleep(interval)
                
        except KeyboardInterrupt:
            print("\n\nMonitoramento interrompido pelo usuário")


def main():
    """Função principal"""
    # Configurar parâmetros de conexão
    HOST = "localhost"  # ou "192.168.1.100" para IP remoto
    PORT = 502
    UNIT_ID = 1
    
    print("="*70)
    print(f"{'CLIENTE MODBUS TCP - MONITOR DE BATERIA':^70}")
    print("="*70)
    
    # Criar cliente
    client = BatteryModbusClient(HOST, PORT, UNIT_ID)
    
    # Conectar
    if not client.connect():
        logger.error("Não foi possível estabelecer conexão com o servidor")
        return
    
    try:
        # Escolher modo de operação
        print("\nModos disponíveis:")
        print("1. Leitura única")
        print("2. Monitoramento contínuo")
        
        choice = input("\nEscolha o modo (1 ou 2): ").strip()
        
        if choice == "1":
            # Leitura única
            print("\nRealizando leitura única...")
            data = client.read_all_data()
            client.print_data(data)
            
        elif choice == "2":
            # Monitoramento contínuo
            interval = input("Intervalo entre leituras (segundos, padrão=2): ").strip()
            interval = int(interval) if interval.isdigit() else 2
            client.monitor_continuous(interval=interval)
        else:
            print("Opção inválida")
            
    except KeyboardInterrupt:
        print("\n\n Programa interrompido")
    except Exception as e:
        logger.error(f" Erro: {e}")
    finally:
        client.disconnect()
        print("\n Encerrando cliente...")


if __name__ == "__main__":
    main()
