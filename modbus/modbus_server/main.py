"""
Servidor Modbus TCP para Disponibilizar Dados de Bateria de Trem
Simula ou coleta dados de aferição e os disponibiliza via Modbus TCP
"""
from pymodbus.server import StartTcpServer
from pymodbus.device import ModbusDeviceIdentification
from pymodbus.datastore import ModbusSequentialDataBlock, ModbusSlaveContext, ModbusServerContext
import threading
import time
import random
import logging

# Configurar logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class BatteryDataProvider:
    """Provedor de dados da bateria (pode ser substituído por leitura real de sensores)"""
    
    def __init__(self):
        # Valores simulados iniciais
        self.voltage = 110.0  # Volts
        self.current = 50.0   # Amperes
        self.temperature = 25.0  # °C
        self.soc = 85.0  # State of Charge (%)
        self.soh = 95.0  # State of Health (%)
        self.cycles = 150  # Número de ciclos
        self.status = 1  # 0=Erro, 1=Normal, 2=Carga, 3=Descarga
        
    def get_voltage(self):
        """Retorna tensão da bateria"""
        # Simula pequenas variações (substitua por leitura real)
        return self.voltage + random.uniform(-2.0, 2.0)
    
    def get_current(self):
        """Retorna corrente da bateria"""
        return self.current + random.uniform(-5.0, 5.0)
    
    def get_temperature(self):
        """Retorna temperatura da bateria"""
        return self.temperature + random.uniform(-1.0, 1.0)
    
    def get_soc(self):
        """Retorna estado de carga"""
        return max(0, min(100, self.soc + random.uniform(-0.5, 0.5)))
    
    def get_soh(self):
        """Retorna estado de saúde"""
        return self.soh
    
    def get_cycles(self):
        """Retorna número de ciclos"""
        return self.cycles
    
    def get_status(self):
        """Retorna status da bateria"""
        return self.status


class ModbusBatteryServer:
    """Servidor Modbus TCP para bateria de trem"""
    
    def __init__(self, host='0.0.0.0', port=502):
        """
        Inicializa o servidor Modbus TCP
        
        Args:
            host: Endereço para bind (0.0.0.0 = todas as interfaces)
            port: Porta TCP (padrão 502, requer root/admin)
        """
        self.host = host
        self.port = port
        self.battery = BatteryDataProvider()
        self.running = False
        
        # Mapeamento de registros Modbus (endereço: descrição)
        self.REGISTER_MAP = {
            0: 'voltage_high',      # Tensão parte alta (16 bits)
            1: 'voltage_low',       # Tensão parte baixa (16 bits)
            2: 'current_high',      # Corrente parte alta
            3: 'current_low',       # Corrente parte baixa
            4: 'temperature',       # Temperatura
            5: 'reserved_1',
            6: 'soc',              # Estado de carga
            7: 'soh',              # Estado de saúde
            8: 'cycles_high',      # Ciclos parte alta
            9: 'cycles_low',       # Ciclos parte baixa
            10: 'status',          # Status
            11: 'reserved_2'
        }
        
        # Criar datastore Modbus
        # Holding Registers (função 03/16): leitura/escrita
        self.store = ModbusSlaveContext(
            hr=ModbusSequentialDataBlock(0, [0]*100)  # 100 registros
        )
        
        self.context = ModbusServerContext(slaves=self.store, single=True)
        
    def float_to_registers(self, value):
        """Converte float em 2 registros de 16 bits"""
        # Multiplica por 100 para manter 2 casas decimais
        int_value = int(value * 100)
        high = (int_value >> 16) & 0xFFFF
        low = int_value & 0xFFFF
        return high, low
    
    def update_registers(self):
        """Atualiza os registros Modbus com dados da bateria"""
        while self.running:
            try:
                # Ler dados da bateria
                voltage = self.battery.get_voltage()
                current = self.battery.get_current()
                temperature = self.battery.get_temperature()
                soc = self.battery.get_soc()
                soh = self.battery.get_soh()
                cycles = self.battery.get_cycles()
                status = self.battery.get_status()
                
                # Converter para registros
                voltage_h, voltage_l = self.float_to_registers(voltage)
                current_h, current_l = self.float_to_registers(current)
                temp_reg = int(temperature * 10)
                soc_reg = int(soc * 10)
                soh_reg = int(soh * 10)
                cycles_h = (cycles >> 16) & 0xFFFF
                cycles_l = cycles & 0xFFFF
                
                # Atualizar registros no datastore
                registers = [
                    voltage_h,    # 0
                    voltage_l,    # 1
                    current_h,    # 2
                    current_l,    # 3
                    temp_reg,     # 4
                    0,            # 5 - reservado
                    soc_reg,      # 6
                    soh_reg,      # 7
                    cycles_h,     # 8
                    cycles_l,     # 9
                    status,       # 10
                    0             # 11 - reservado
                ]
                
                # Escrever no contexto Modbus
                self.context[0].setValues(3, 0, registers)
                
                # Log dos valores atualizados
                logger.info(f"Dados atualizados - V: {voltage:.2f}V, "
                           f"I: {current:.2f}A, T: {temperature:.1f}°C, "
                           f"SoC: {soc:.1f}%, SoH: {soh:.1f}%")
                
                time.sleep(1)  # Atualiza a cada 1 segundo
                
            except Exception as e:
                logger.error(f"Erro ao atualizar registros: {e}")
                time.sleep(1)
    
    def start(self):
        """Inicia o servidor Modbus"""
        self.running = True
        
        # Thread para atualizar dados continuamente
        update_thread = threading.Thread(target=self.update_registers, daemon=True)
        update_thread.start()
        
        # Configurar identificação do dispositivo
        identity = ModbusDeviceIdentification()
        identity.VendorName = 'Sistema Ferroviário'
        identity.ProductCode = 'BATT-MONITOR'
        identity.VendorUrl = 'http://github.com/battery-monitor'
        identity.ProductName = 'Monitor de Bateria de Trem'
        identity.ModelName = 'BMT-1000'
        identity.MajorMinorRevision = '1.0.0'
        
        logger.info(f"Iniciando servidor Modbus TCP em {self.host}:{self.port}")
        logger.info("Mapeamento de registros:")
        logger.info("  [0-1]  Tensão (V) - 2 registros (float*100)")
        logger.info("  [2-3]  Corrente (A) - 2 registros (float*100)")
        logger.info("  [4]    Temperatura (°C*10)")
        logger.info("  [6]    Estado de Carga (%*10)")
        logger.info("  [7]    Estado de Saúde (%*10)")
        logger.info("  [8-9]  Ciclos - 2 registros")
        logger.info("  [10]   Status (0=Erro, 1=Normal, 2=Carga, 3=Descarga)")
        logger.info("\nServidor rodando. Pressione Ctrl+C para parar.")
        
        try:
            # Iniciar servidor (bloqueante)
            StartTcpServer(
                context=self.context,
                identity=identity,
                address=(self.host, self.port)
            )
        except KeyboardInterrupt:
            logger.info("\nServidor interrompido pelo usuário")
        except Exception as e:
            logger.error(f"Erro no servidor: {e}")
        finally:
            self.running = False
    
    def stop(self):
        """Para o servidor"""
        self.running = False
        logger.info("Servidor parado")


# Exemplo de uso
if __name__ == "__main__":
    # Criar servidor
    # Nota: porta 502 requer privilégios root/admin
    # Use porta > 1024 (ex: 5020) para executar sem privilégios
    server = ModbusBatteryServer(host='0.0.0.0', port=502)
    
    try:
        server.start()
    except KeyboardInterrupt:
        server.stop()
