# DModbus: Blockchain for Network Identity Validation in Modbus Connections

**Manuscript ID:** 10703

**Authors:**

- Paulo Henrique Mariano — Universidade Federal de Ouro Preto (UFOP) / Federation of Industries of the State of Minas Gerais (FIEMG), Brazil
- Devanir Caetano Filho — Federation of Industries of the State of Minas Gerais (FIEMG) / SENAI Center 4.0, Contagem, MG, Brazil
- Carlos Frederico Cunha Cavalcanti — Universidade Federal de Ouro Preto (UFOP), Ouro Preto, MG, Brazil
- Ricardo Augusto Rabelo Oliveira — Universidade Federal de Ouro Preto (UFOP), Ouro Preto, MG, Brazil

---

## About

This repository contains the official implementation of **DModbus**, a non-invasive retrofit security layer that mitigates inherent vulnerabilities in legacy industrial networks running the Modbus/TCP protocol. DModbus introduces a gateway that intercepts Modbus traffic and validates each device's identity against an immutable record stored on a permissioned Proof of Authority (PoA) blockchain. A mismatch between the trusted ledger entry and the live ARP table triggers a real-time security alert, successfully neutralizing Man-in-the-Middle (MitM) and ARP spoofing attacks without requiring any firmware changes to existing equipment.

The prototype was validated through a Proof of Concept (TRL 3) experiment simulating an ARP spoofing MitM attack in a controlled Modbus TCP/IP environment. Experimental results show that DModbus detects identity spoofing in real time with a mean latency overhead of 91.3% relative to an unprotected baseline — a controlled and stable cost compared to the severe, unpredictable degradation observed during active attacks (peak latency of 57.8 ms).

---

## Repository Structure

```text
dmodbus3/
├── attack_simulation/
│   └── attack_simulation.py
├── blockchain_template/
│   ├── api/
│   │   ├── handler.go
│   │   └── server.go
│   ├── arp/
│   │   ├── arp.go
│   │   └── arp_test.go
│   ├── cmd/
│   │   └── node/
│   │       └── main.go
│   ├── internal/
│   │   └── blockchain/
│   │       ├── block.go
│   │       ├── blockchain.go
│   │       └── *_test.go
│   ├── network/
│   │   ├── peer.go
│   │   └── peer_test.go
│   ├── go.mod
│   └── go.sum
├── gateway/
│   ├── gateway.py
│   └── requirements.txt
├── modbus/
│   ├── modbus_client/
│   │   └── main.py
│   └── modbus_server/
│       └── main.py
└── README.md
```

---

## File Descriptions

### `attack_simulation/attack_simulation.py`
Unified Scapy-based script that simulates the MitM adversary used in Scenarios 2 and 4. It performs ARP spoofing to position the attacker between the Sensor and SCADA nodes, then intercepts and modifies Modbus TCP packets in transit, injecting spoofed Function Code 0x03 payloads. Run this on a separate machine (or VM) representing the compromised node on the link layer.

---

### `blockchain_template/` — Permissioned PoA Ledger (Go)

| File | Description |
|---|---|
| `api/handler.go` | REST API endpoint handlers for ledger synchronisation, device record queries, and audit log retrieval. |
| `api/server.go` | HTTP web server with route mappings; binds to port 8080 by default. |
| `arp/arp.go` | Parses the local operating-system ARP table, returning a MAC-to-IP mapping for live network state comparison. |
| `arp/arp_test.go` | Unit tests for the ARP table parsing logic. |
| `cmd/node/main.go` | Main entry point. Compile and run this file to start a validator node. The node joins the P2P cluster, participates in PoA consensus, and exposes the REST API. |
| `internal/blockchain/block.go` | Core block structure: fields, SHA-256 hashing, and JSON serialisation/deserialisation. |
| `internal/blockchain/blockchain.go` | LevelDB persistence layer: chain initialisation, block appending, consensus validation, and inter-node synchronisation. |
| `internal/blockchain/*_test.go` | Cryptographic consensus unit tests covering block hashing, chain integrity, and quorum calculation. |
| `network/peer.go` | RWMutex-protected peer discovery and block broadcasting; manages the list of known validator nodes in the permissioned cluster. |
| `network/peer_test.go` | Unit tests for P2P cluster connection and broadcasting assertions. |
| `go.mod` | Go module definition (module name and minimum Go version). |
| `go.sum` | Dependency checksum manifest for reproducible builds. |

---

### `gateway/` — Non-Invasive Inspection Gateway (Python)

| File | Description |
|---|---|
| `gateway.py` | Passive network sniffer that captures live Modbus TCP packets. For each packet, it queries the local blockchain ledger via REST and compares the trusted MAC address with the current ARP table entry for that IP. A mismatch prints a `[ALERT!] ATTACK DETECTED!` warning to stdout. Corresponds to Scenarios 3 and 4 in the paper. |
| `requirements.txt` | Python package dependencies (`scapy`, `requests`, etc.) for the gateway component. |

---

### `modbus/` — Modbus Simulator (Python)

| File | Description |
|---|---|
| `modbus_client/main.py` | SCADA client simulator. Polls holding registers from the Modbus server at regular intervals using the `pymodbus` library. Represents the SCADA node in all four experimental scenarios. |
| `modbus_server/main.py` | Sensor server simulator. Hosts a set of holding registers over Modbus TCP. Represents the field device (sensor) in all four experimental scenarios. |

---

## Requirements

### Blockchain node (Go)
- Go 1.20 or later
- Dependencies are managed via `go.mod`/`go.sum`; no manual installation required

### Gateway and Modbus simulators (Python)
- Python 3.9 or later
- Install dependencies with:

```bash
cd gateway
pip install -r requirements.txt
```

### Attack simulation
- A Kali Linux machine (or any Linux host with root access and Scapy installed)
- Network-level access between attacker, sensor, and SCADA nodes

---

## How to Reproduce the Experiments

The four experimental scenarios described in the paper can be reproduced as follows.

### Step 1 — Start the Modbus devices

Open two terminals and run the sensor and SCADA simulators:

```bash
# Terminal 1 – Sensor (Modbus server)
cd modbus/modbus_server
python main.py

# Terminal 2 – SCADA (Modbus client)
cd modbus/modbus_client
python main.py
```

This corresponds to **Scenario 1 (Baseline)**.

### Step 2 — Compile and start the blockchain node

Build the Go binary locally to avoid execution blocks from host security tools:

```bash
cd blockchain_template/cmd/node
go build -o dmodbus.exe main.go
./dmodbus.exe
```

The validator node starts an HTTP REST service on port 8080 and begins participating in PoA consensus. Keep this terminal running.

### Step 3 — Start the gateway

Run the gateway with administrative privileges so it can bind to raw sockets. Pass the network interface name as an argument (defaults to `Ethernet` on Windows if omitted):

```bash
cd gateway
python gateway.py eth0
```

Modbus traffic is now being validated against the blockchain ledger. This corresponds to **Scenario 3 (DModbus active, no attack)**.

### Step 4 — Launch the attack simulation

On a separate machine or VM (the attacker node), run:

```bash
cd attack_simulation
python attack_simulation.py eth0
```

The script performs ARP spoofing and injects spoofed Modbus packets. Without the gateway running, this is **Scenario 2 (MitM attack)**. With the gateway running simultaneously, this is **Scenario 4 (DModbus under attack)** — the gateway will print `[ALERT!] ATTACK DETECTED!` messages as it detects the MAC address mismatch.

---

## Experimental Results Summary

The table below reproduces the latency statistics reported in the paper (3,007 samples per scenario, values in milliseconds):

| Metric (ms) | Scenario 1 — Baseline | Scenario 2 — MitM Attack | Scenario 3 — DModbus | Scenario 4 — DModbus + MitM | Modbus TLS |
|:---|:---:|:---:|:---:|:---:|:---:|
| Mean | 0.700 | 2.406 | 1.339 | 2.643 | 1.408 |
| Median | 0.508 | 1.830 | 0.776 | 1.890 | 0.889 |
| Std. Deviation | 0.639 | 2.274 | 1.799 | 2.479 | 1.511 |
| Minimum | 0.258 | 0.273 | 0.258 | 0.267 | 0.269 |
| Maximum | 7.386 | 57.830 | 22.497 | 36.393 | 18.482 |

DModbus introduces a **91.3% mean latency overhead** relative to the unprotected baseline, which is lower than the official Modbus TLS overhead (101%) and remains well below the 100 ms application timeouts typical in SCADA supervisory systems. During an active MitM attack, the unprotected network reaches peak latencies of 57.8 ms; DModbus caps this at 22.5 ms while simultaneously triggering alerts.

---

## Contact

For questions or to report issues with result reproduction, please open a GitHub Issue or contact the corresponding author:

**Paulo Henrique Mariano** — paulo.hm@aluno.ufop.edu.br
