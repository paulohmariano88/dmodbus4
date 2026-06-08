# DModbus: Blockchain for Network Identity Validation in Modbus Connections

This repository contains the official implementation of **DModbus**, a non-invasive retrofit security layer designed to mitigate inherent vulnerabilities in legacy industrial automation networks operating under the Modbus/TCP protocol. By integrating a permissioned Proof of Authority (PoA) blockchain framework, DModbus builds a distributed and immutable ledger for device identities. [cite_start]This distributed notary infrastructure enables robust, real-time cryptographic validation of network endpoints, successfully neutralizing Man-in-the-Middle (MitM) and Address Resolution Protocol (ARP) spoofing vectors.

---
## Authors
* **Paulo Henrique Mariano** (Federal University of Ouro Preto / FIEMG) 
* **Devanir Caetano Filho** (FIEMG System / SENAI Center 4.0) 
* **Carlos Frederico Cunha Cavalcanti** (Federal University of Ouro Preto)
* **Ricardo Augusto Rabelo Oliveira** (Federal University of Ouro Preto)

---

## Project Structure

The codebase is organized into three decoupled architectural directories, optimizing the operational boundaries between the core blockchain layer, the localized filtering gateway, and the penetration testing vectors:

```text
dmodbus3/
├── attack_simulation/
│   └── attack_simulation.py     # Unified Scapy packet injection script (Scenario 2 & 4)
├── blockchain_template/         # Permissioned PoA Ledger Core Engine (Go Core)
│   ├── api/
│   │   ├── handler.go           # REST API endpoints for ledger sync, query, and auditing
│   │   └── server.go            # HTTP web server routing mappings
│   ├── arp/
│   │   ├── arp.go               # Local OS raw ARP table indexing parser
│   │   └── arp_test.go          # Unit tests for network table parsing
│   ├── cmd/
│   │   └── node/
│   │       └── main.go          # Main entry point to compile and run the validator node
│   ├── internal/
│   │   └── blockchain/
│   │       ├── block.go         # Core block structure definitions, serialization, and hashing
│   │       ├── blockchain.go    # LevelDB transactions, synchronization, and chain validations
│   │       └── *_test.go        # Cryptographic consensus unit testing routines
│   ├── network/
│   │   ├── peer.go              # RWMutex-protected P2P cluster discovery and broadcasting
│   │   └── peer_test.go         # P2P connection cluster assertions
│   ├── go.mod                   # Go module definitions
│   └── go.sum                   # Go dependencies checksum validation manifest
├── gateway/                     # Non-Invasive Passive Interception Gateway (Python Component)
│   ├── gateway.py               # Live network sniffer cross-checking packets against Go ledger
│   └── requirements.txt         # Package dependencies for the monitoring subsystem
├── modbus/                      # Module with the modbus simulate system
|   ├── modbus_client            # Client Modbus Simulator
│   |   └── main.py              # SCADA Client Simulator polling registers
|   ├── modbus_server            # Server Modbus Simulator
│       └── main.py              # Sensor Server Simulator hosting holding registers 
└── README.md                    # System-wide reproduction guidelines (this file)

```

## Theoretical & Consensus Framework

To ensure determinism and the low-latency profiles required inside industrial operational technology (OT) parameters, DModbus shifts the computational overhead away from energy-expensive consensus strategies towards a lightweight private PoA model.

### Consensus Threshold (Quorum Validation)

The mathematical model governing ledger updates requires a strict majority transaction validation threshold[cite: 239, 241]. [cite_start]Given a total validator population $N$ within the permissioned cluster, the consensus quorum boundary $T$ is formalized by:

$$T = \lfloor\frac{N}{2}\rfloor + 1$$

This architectural baseline implements Byzantine Fault Tolerance optimized for device identity notary services, maintaining cryptographic ledger immutability as long as the collection of honest nodes satisfies this strict majority index.

### Computational Complexity Analysis

* **Live Ingestion Monitoring:** The gateway identity validation runtime scales at a constant complexity of $O(1)$, because packet verification executes a direct key-value index query against the local synchronized state of the ledger, preventing execution overhead from degrading with network size.
* **Administrative State Modifications:** Registering or provisioning new device identities scales linearly at a complexity of $O(N)$, where $N$ represents the active validator nodes required to reach consensus agreement.

## Performance Evaluation & Benchmarks

The DModbus architecture was validated across a dataset of 3,007 transaction cycles per scenario to map the exact latency trade-offs introduced by decentralized identity notary services:  

### Latency Metric Matrix

The quantitative evaluation benchmarks collected during experimental trials are structured below (values measured in milliseconds):

| Latency Metric (ms) | Scenario 1 (Baseline) | Scenario 2 (MitM Attack) | Scenario 3 (DModbus Active) | Scenario 4 (DModbus Under Attack) | Modbus/TLS (Official Standard) |
| :--- | :---: | :---: | :---: | :---: | :---: |
| **Mean** | 0.700 | 2.406 | 1.339 | 2.643 | 1.408 |
| **Median** | 0.508 | 1.830 | 0.776 | 1.890 | 0.889 |
| **Standard Deviation** | 0.639 | 2.274 | 1.799 | 2.479 | 1.511 |
| **Minimum** | 0.258 | 0.273 | 0.258 | 0.267 | 0.269 |
| **Maximum** | 7.386 | 57.830 | 22.497 | 36.393 | 18.482 |

### Operational Analysis Summary

* **Performance Overhead:** The integration of the DModbus validation framework introduces a 91.3% mean latency overhead relative to unprotected native communication. This delay is controlled, stable, and remains well below standard web-based SCADA delays or application execution timeouts that regularly exceed 100 ms in real-world OT scenarios.  
* **Comparative Resilience:** DModbus incurs a lower latency penalty (91.3%) compared to the official Modbus/TLS framework (101% overhead), while eliminating the administrative complexity and single point of failure risks typical of centralized Public Key Infrastructures (PKI).

## Step-by-Step Reproduction Guide

Follow these sequential execution steps to provision the network, simulate the adversary threat, and verify defensive alerting outputs:

### 1. Environment Preparation
Navigate into the gateway sub-directory and install the standard network packet manipulation requirements:
```bash
cd gateway
pip install -r requirements.txt
```

### 2. Compile and Initialize the Ledger Node 

To prevent runtime execution or heuristic access blocks from local host security tools, compile the Go binary locally inside your workspace environment instead of running from a temp path directory:

```bash
cd ../blockchain_template/cmd/node
go build -o dmodbus.exe main.go
```
Keep this terminal window running. The validator node instantiates its HTTP REST backend service listening on port 8080.


### 3. Deploy the Non-Invasive Inspection Gateway

Launch the sniffing component with administrative privileges to bind to raw socket frames. 
You can pass the specific interface name as an argument (defaults to "Ethernet" for Windows configurations if empty):


```bash
cd ../../../gateway
python gateway.py eth0
```

## Launch the Penetration Vector (Threat Agent Simulation)

Open a separate administrative terminal window representing the compromised node on the link layer. Run the unified attack script to intercept traffic and attempt injection of spoofed Modbus Function Code 0x03 payloads:

```bash
cd ../attack_simulation
python attack_simulation.py eth0
``` 
