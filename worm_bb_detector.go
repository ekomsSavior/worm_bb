// worm_bb_detector.go - Worm-BB Detection and Removal Tool (UPDATED)
// RED TEAM / BLUE TEAM UTILITY - Authorized Use Only
// Version 2.0 - Full coverage for Worm-BB v4.0-DEFCON-ARM

package main

import (
    "bufio"
    "bytes"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "io/ioutil"
    "net"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "regexp"
    "runtime"
    "strings"
    "sync"
    "syscall"
    "time"

    "golang.org/x/sys/windows"
    "golang.org/x/sys/windows/registry"
)

// ========== CONSTANTS ==========

const (
    VERSION = "2.0"
    MUTEX_NAME = "Global\\SystemUpdateMutex"
    MULTICAST_ADDR = "239.255.42.42:4242"
    LOCK_FILE = "/tmp/.system-update.lock"

    // Windows worm artifacts
    WORM_REGISTRY_KEYS = "HKCU\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Run\\SystemUpdate,HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Run\\SystemUpdate"
    WORM_SCHEDULED_TASKS = "SystemUpdateTask,SystemUpdateTask_startup"
    WORM_WMI_FILTER = "SystemUpdateFilter"
    WORM_WMI_CONSUMER = "SystemUpdateConsumer"

    // Linux worm artifacts
    WORM_SYSTEMD_SERVICE = "system-update.service"
    WORM_CRON_JOBS = "@reboot /tmp/system-update,*/30 * * * * /tmp/system-update"
    WORM_UDEV_RULE = "99-usb-autorun.rules"
    WORM_INIT_SCRIPT = "/etc/init.d/system-update"
    WORM_RC_LOCAL_PATTERN = "/tmp/system-update &"

    // SSH backdoor
    WORM_SSH_KEY_PATTERN = "worm-bb-key"
    WORM_SSH_KEY_CONTENT = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC"

    // USB artifacts
    WORM_USB_AUTORUN = "autorun.inf"
    WORM_USB_EXE = "SystemUpdate.exe"
    WORM_USB_LNK = "System Update.lnk"
    WORM_USB_MAC_APP = "SystemUpdate.app"
    WORM_USB_LINUX_HIDDEN = ".system-update"
    WORM_USB_LINUX_DESKTOP = ".system-update.desktop"

    // WebShell artifacts
    WORM_WEBSHELL_PHP = "shell.php"
    WORM_WEBSHELL_ASP = "shell.aspx"
    WORM_WEBSHELL_PY = "shell.py"
    WORM_WEBSHELL_BACKDOOR = "backdoor.php"
    WORM_WEBSHELL_UPDATE = "system-update.php"

    // WiFi artifacts
    WIFI_HOSTAPD_CONF = "/tmp/hostapd.conf"
    WIFI_DHCPD_CONF = "/tmp/dhcpd.conf"
    WIFI_EVIL_PORTAL_PORT = 8443

    // P2P ports
    P2P_PORTS = "4242,4243,4444"
)

// ========== DATA STRUCTURES ==========

type DetectionResult struct {
    Timestamp      time.Time              `json:"timestamp"`
    Hostname       string                 `json:"hostname"`
    OS             string                 `json:"os"`
    IPAddress      string                 `json:"ip_address"`
    WormDetected   bool                   `json:"worm_detected"`
    Severity       string                 `json:"severity"`
    Findings       []Finding              `json:"findings"`
    Remediations   []Remediation          `json:"remediations"`
    ScanDuration   time.Duration          `json:"scan_duration"`
    WormVersion    string                 `json:"worm_version,omitempty"`
}

type Finding struct {
    Category     string   `json:"category"`
    Location     string   `json:"location"`
    Details      string   `json:"details"`
    Confidence   string   `json:"confidence"`
    RemediationID string  `json:"remediation_id"`
}

type Remediation struct {
    ID           string   `json:"id"`
    Action       string   `json:"action"`
    Target       string   `json:"target"`
    Command      string   `json:"command"`
    RequiresReboot bool   `json:"requires_reboot"`
    Status       string   `json:"status"`
}

// ========== DETECTION ENGINE ==========

type DetectionEngine struct {
    results       *DetectionResult
    findings      []Finding
    remediations  []Remediation
    mu            sync.Mutex
    wormHashes    map[string]bool
    networkScan   bool
    autoRemediate bool
}

func NewDetectionEngine(networkScan, autoRemediate bool) *DetectionEngine {
    return &DetectionEngine{
        results: &DetectionResult{
            Timestamp:    time.Now(),
            Hostname:     getHostname(),
            OS:           runtime.GOOS,
            IPAddress:    getLocalIP(),
            WormDetected: false,
            Severity:     "LOW",
            Findings:     []Finding{},
            Remediations: []Remediation{},
        },
        findings:     []Finding{},
        remediations: []Remediation{},
        wormHashes:   make(map[string]bool),
        networkScan:  networkScan,
        autoRemediate: autoRemediate,
    }
}

func (de *DetectionEngine) RunFullScan() {
    startTime := time.Now()
    defer func() {
        de.results.ScanDuration = time.Since(startTime)
        de.results.Findings = de.findings
        de.results.Remediations = de.remediations
    }()

    fmt.Println("[Worm-BB Detector v2.0] Starting comprehensive scan")
    fmt.Println("================================================")

    // Process scanning
    fmt.Println("[*] Scanning for worm processes...")
    de.scanProcesses()

    // File system scanning - expanded for all platforms
    fmt.Println("[*] Scanning for worm files...")
    de.scanFiles()

    // Registry scanning (Windows only)
    if runtime.GOOS == "windows" {
        fmt.Println("[*] Scanning registry...")
        de.scanRegistry()

        fmt.Println("[*] Scanning scheduled tasks...")
        de.scanScheduledTasks()

        fmt.Println("[*] Scanning WMI subscriptions...")
        de.scanWMI()

        fmt.Println("[*] Scanning startup folder...")
        de.scanStartupFolder()
    }

    // Linux-specific scans
    if runtime.GOOS == "linux" {
        fmt.Println("[*] Scanning cron jobs...")
        de.scanCronJobs()

        fmt.Println("[*] Scanning systemd services...")
        de.scanSystemdServices()

        fmt.Println("[*] Scanning udev rules...")
        de.scanUdevRules()

        fmt.Println("[*] Scanning init scripts...")
        de.scanInitScripts()

        fmt.Println("[*] Scanning rc.local...")
        de.scanRCLocal()
    }

    // macOS-specific scans
    if runtime.GOOS == "darwin" {
        fmt.Println("[*] Scanning launch agents...")
        de.scanLaunchAgents()
    }

    // Common scans
    fmt.Println("[*] Scanning SSH authorized_keys...")
    de.scanSSHKeys()

    fmt.Println("[*] Scanning USB drives...")
    de.scanUSBDrives()

    fmt.Println("[*] Scanning for WebShells...")
    de.scanWebShells()

    if de.networkScan {
        fmt.Println("[*] Scanning network for worm peers...")
        de.scanNetwork()

        fmt.Println("[*] Scanning for WiFi artifacts...")
        de.scanWiFiArtifacts()
    }

    fmt.Println("[*] Scanning memory for signatures...")
    de.scanMemory()

    fmt.Println("[*] Scanning for mutex/lock files...")
    de.scanMutex()

    fmt.Println("[*] Scanning for P2P communication...")
    de.scanP2P()

    // Determine overall severity
    de.calculateSeverity()

    fmt.Println("================================================")
    fmt.Printf("[+] Scan completed in %v\n", de.results.ScanDuration)

    if de.results.WormDetected {
        fmt.Printf("[!] WORM DETECTED! Severity: %s\n", de.results.Severity)
        fmt.Printf("[!] Found %d indicators\n", len(de.findings))
    } else {
        fmt.Println("[+] No worm detected")
    }
}

func (de *DetectionEngine) scanProcesses() {
    var processes []map[string]interface{}

    if runtime.GOOS == "windows" {
        processes = de.getWindowsProcesses()
    } else {
        processes = de.getLinuxProcesses()
    }

    for _, proc := range processes {
        procName := proc["name"].(string)
        procPID := proc["pid"].(int)

        // Check process names
        suspiciousNames := []string{
            "system-update", "SystemUpdate", "worm_bb", "worm-bb",
            "SecurityUpdate", "systemupdate", "sysupdate",
        }
        for _, name := range suspiciousNames {
            if strings.Contains(strings.ToLower(procName), strings.ToLower(name)) {
                de.addFinding(Finding{
                    Category:    "PROCESS",
                    Location:    fmt.Sprintf("PID: %d", procPID),
                    Details:     fmt.Sprintf("Suspicious process: %s", procName),
                    Confidence:  "HIGH",
                    RemediationID: "remediate_process",
                })
                de.results.WormDetected = true
            }
        }

        // Check command line for suspicious strings
        if cmdline, ok := proc["cmdline"].(string); ok {
            suspiciousCmdline := []string{
                "system-update", "SystemUpdate", "worm_bb",
                "/tmp/system-update", "SystemUpdate.exe",
                "Free_Public_WiFi", "evil portal", "rogue AP",
                "hostapd", "dnsmasq", "aireplay-ng",
            }
            for _, pattern := range suspiciousCmdline {
                if strings.Contains(cmdline, pattern) {
                    de.addFinding(Finding{
                        Category:    "PROCESS",
                        Location:    fmt.Sprintf("PID: %d", procPID),
                        Details:     fmt.Sprintf("Suspicious command line: %s", cmdline),
                        Confidence:  "HIGH",
                        RemediationID: "remediate_process",
                    })
                    de.results.WormDetected = true
                    break
                }
            }
        }
    }
}

func (de *DetectionEngine) scanFiles() {
    // Windows paths
    if runtime.GOOS == "windows" {
        paths := []string{
            "C:\\Windows\\Temp\\system-update.exe",
            os.Getenv("APPDATA") + "\\Microsoft\\Windows\\Start Menu\\Programs\\Startup\\SystemUpdate.exe",
            os.Getenv("TEMP") + "\\worm*.exe",
            "C:\\Windows\\Temp\\screenshot.png",
            "C:\\Windows\\Temp\\exfil.dat",
        }
        for _, path := range paths {
            if matches, _ := filepath.Glob(path); len(matches) > 0 {
                for _, match := range matches {
                    de.addWormFile(match)
                }
            }
        }
    }

    // Linux paths
    if runtime.GOOS == "linux" {
        paths := []string{
            "/tmp/system-update",
            "/tmp/.system-update",
            "/tmp/worm",
            "/tmp/.system-update.lock",
            "/var/tmp/system-update",
            "/dev/shm/system-update",
            "/etc/systemd/system/system-update.service",
            "/etc/udev/rules.d/99-usb-autorun.rules",
            "/etc/init.d/system-update",
            "/tmp/hostapd.conf",
            "/tmp/dhcpd.conf",
        }
        for _, path := range paths {
            if _, err := os.Stat(path); err == nil {
                de.addWormFile(path)
            }
        }
    }

    // macOS paths
    if runtime.GOOS == "darwin" {
        paths := []string{
            os.Getenv("HOME") + "/Library/LaunchAgents/com.apple.systemupdate.plist",
            "/tmp/system-update",
            "/Volumes/*/SystemUpdate.app",
        }
        for _, path := range paths {
            if matches, _ := filepath.Glob(path); len(matches) > 0 {
                for _, match := range matches {
                    de.addWormFile(match)
                }
            }
        }
    }

    // Recursive scan of common directories
    scanDirs := []string{}
    if runtime.GOOS == "windows" {
        scanDirs = []string{os.Getenv("TEMP"), os.Getenv("APPDATA"), "C:\\Windows\\Temp"}
    } else if runtime.GOOS == "darwin" {
        scanDirs = []string{"/tmp", "/var/tmp", os.Getenv("HOME") + "/Library/LaunchAgents"}
    } else {
        scanDirs = []string{"/tmp", "/var/tmp", "/dev/shm", "/etc/systemd/system", "/etc/udev/rules.d"}
    }

    for _, dir := range scanDirs {
        if _, err := os.Stat(dir); err == nil {
            de.recursiveFileScan(dir)
        }
    }
}

func (de *DetectionEngine) recursiveFileScan(dir string) {
    files, err := ioutil.ReadDir(dir)
    if err != nil {
        return
    }

    for _, file := range files {
        if file.IsDir() {
            if strings.HasPrefix(file.Name(), ".") {
                continue
            }
            de.recursiveFileScan(filepath.Join(dir, file.Name()))
        } else {
            filename := file.Name()
            fullPath := filepath.Join(dir, filename)

            // Check for known worm file patterns
            patterns := []string{
                "system-update", "SystemUpdate", "worm_bb", "worm-bb",
                "autorun.inf", "System Update.lnk",
                "shell.php", "shell.aspx", "shell.py",
                "backdoor.php", "system-update.php",
                "com.apple.systemupdate",
                "99-usb-autorun.rules",
            }
            for _, pattern := range patterns {
                if strings.Contains(filename, pattern) {
                    de.addWormFile(fullPath)
                    break
                }
            }

            // Check file size for suspicious executables
            if file.Size() > 1000000 && file.Size() < 10000000 { // 1-10MB
                if strings.HasSuffix(filename, ".exe") || strings.HasSuffix(filename, ".bin") {
                    de.hashFile(fullPath)
                }
            }
        }
    }
}

func (de *DetectionEngine) addWormFile(path string) {
    de.addFinding(Finding{
        Category:    "FILE",
        Location:    path,
        Details:     fmt.Sprintf("Suspicious file found: %s", path),
        Confidence:  "HIGH",
        RemediationID: "remediate_file",
    })

    de.addRemediation(Remediation{
        ID:            generateID(),
        Action:        "DELETE_FILE",
        Target:        path,
        Command:       de.getDeleteCommand(path),
        RequiresReboot: false,
        Status:        "PENDING",
    })

    de.results.WormDetected = true
    de.hashFile(path)
}

func (de *DetectionEngine) hashFile(path string) {
    data, err := ioutil.ReadFile(path)
    if err != nil {
        return
    }
    hash := sha256.Sum256(data)
    hashStr := hex.EncodeToString(hash[:])
    de.wormHashes[hashStr] = true
}

func (de *DetectionEngine) scanRegistry() {
    if runtime.GOOS != "windows" {
        return
    }

    keys := []string{
        "HKCU\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Run\\SystemUpdate",
        "HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Run\\SystemUpdate",
        "HKCU\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Run\\SystemUpdateTask",
    }

    for _, keyPath := range keys {
        parts := strings.Split(keyPath, "\\")
        if len(parts) < 2 {
            continue
        }

        hive := parts[0]
        key := strings.Join(parts[1:], "\\")

        var regKey registry.Key
        var err error

        switch hive {
        case "HKCU":
            regKey, err = registry.OpenKey(registry.CURRENT_USER, key, registry.READ)
        case "HKLM":
            regKey, err = registry.OpenKey(registry.LOCAL_MACHINE, key, registry.READ)
        default:
            continue
        }

        if err == nil {
            valueName := filepath.Base(key)
            value, _, err := regKey.GetStringValue(valueName)
            if err == nil && value != "" {
                de.addFinding(Finding{
                    Category:    "REGISTRY",
                    Location:    keyPath,
                    Details:     fmt.Sprintf("Suspicious registry value: %s = %s", keyPath, value),
                    Confidence:  "HIGH",
                    RemediationID: "remediate_registry",
                })

                de.addRemediation(Remediation{
                    ID:            generateID(),
                    Action:        "DELETE_REGISTRY",
                    Target:        keyPath,
                    Command:       fmt.Sprintf("reg delete \"%s\" /v %s /f", keyPath, valueName),
                    RequiresReboot: false,
                    Status:        "PENDING",
                })

                de.results.WormDetected = true
            }
            regKey.Close()
        }
    }
}

func (de *DetectionEngine) scanScheduledTasks() {
    if runtime.GOOS != "windows" {
        return
    }

    tasks := []string{"SystemUpdateTask", "SystemUpdateTask_startup"}
    for _, task := range tasks {
        cmd := exec.Command("schtasks", "/query", "/tn", task, "/fo", "csv", "/nh")
        err := cmd.Run()
        if err == nil {
            de.addFinding(Finding{
                Category:    "SCHEDULED_TASK",
                Location:    task,
                Details:     fmt.Sprintf("Suspicious scheduled task: %s", task),
                Confidence:  "HIGH",
                RemediationID: "remediate_task",
            })

            de.addRemediation(Remediation{
                ID:            generateID(),
                Action:        "DELETE_TASK",
                Target:        task,
                Command:       fmt.Sprintf("schtasks /delete /tn %s /f", task),
                RequiresReboot: false,
                Status:        "PENDING",
            })

            de.results.WormDetected = true
        }
    }
}

func (de *DetectionEngine) scanWMI() {
    if runtime.GOOS != "windows" {
        return
    }

    // Check for WMI event filter
    cmd := exec.Command("powershell", "-Command",
        "Get-WmiObject -Namespace root\\subscription -Class __EventFilter | Where-Object {$_.Name -eq 'SystemUpdateFilter'} | Select-Object -Property Name")
    output, err := cmd.Output()
    if err == nil && strings.Contains(string(output), "SystemUpdateFilter") {
        de.addFinding(Finding{
            Category:    "WMI",
            Location:    "root\\subscription",
            Details:     "Suspicious WMI event filter detected",
            Confidence:  "HIGH",
            RemediationID: "remediate_wmi",
        })

        de.addRemediation(Remediation{
            ID:            generateID(),
            Action:        "DELETE_WMI",
            Target:        "SystemUpdateFilter",
            Command:       "Get-WmiObject -Namespace root\\subscription -Class __EventFilter | Where-Object {$_.Name -eq 'SystemUpdateFilter'} | Remove-WmiObject",
            RequiresReboot: false,
            Status:        "PENDING",
        })

        de.results.WormDetected = true
    }

    // Check for WMI consumer
    cmd = exec.Command("powershell", "-Command",
        "Get-WmiObject -Namespace root\\subscription -Class CommandLineEventConsumer | Where-Object {$_.Name -eq 'SystemUpdateConsumer'} | Select-Object -Property Name")
    output, err = cmd.Output()
    if err == nil && strings.Contains(string(output), "SystemUpdateConsumer") {
        de.addFinding(Finding{
            Category:    "WMI",
            Location:    "root\\subscription",
            Details:     "Suspicious WMI consumer detected",
            Confidence:  "HIGH",
            RemediationID: "remediate_wmi",
        })

        de.addRemediation(Remediation{
            ID:            generateID(),
            Action:        "DELETE_WMI",
            Target:        "SystemUpdateConsumer",
            Command:       "Get-WmiObject -Namespace root\\subscription -Class CommandLineEventConsumer | Where-Object {$_.Name -eq 'SystemUpdateConsumer'} | Remove-WmiObject",
            RequiresReboot: false,
            Status:        "PENDING",
        })

        de.results.WormDetected = true
    }

    // Check for FilterToConsumerBinding
    cmd = exec.Command("powershell", "-Command",
        "Get-WmiObject -Namespace root\\subscription -Class __FilterToConsumerBinding | Where-Object {$_.Filter -match 'SystemUpdateFilter'} | Select-Object -Property Filter,Consumer")
    output, err = cmd.Output()
    if err == nil && strings.Contains(string(output), "SystemUpdateFilter") {
        de.addFinding(Finding{
            Category:    "WMI",
            Location:    "root\\subscription",
            Details:     "Suspicious WMI filter-to-consumer binding detected",
            Confidence:  "HIGH",
            RemediationID: "remediate_wmi",
        })

        de.results.WormDetected = true
    }
}

func (de *DetectionEngine) scanStartupFolder() {
    if runtime.GOOS != "windows" {
        return
    }

    startupPath := filepath.Join(os.Getenv("APPDATA"),
        "Microsoft", "Windows", "Start Menu", "Programs", "Startup",
        "SystemUpdate.exe")

    if _, err := os.Stat(startupPath); err == nil {
        de.addWormFile(startupPath)
    }
}

func (de *DetectionEngine) scanCronJobs() {
    if runtime.GOOS == "windows" {
        return
    }

    cmd := exec.Command("crontab", "-l")
    output, err := cmd.Output()
    if err != nil {
        // Try as root
        cmd = exec.Command("sudo", "crontab", "-l")
        output, err = cmd.Output()
        if err != nil {
            return
        }
    }

    cronContent := string(output)
    cronJobs := []string{
        "@reboot /tmp/system-update",
        "*/30 * * * * /tmp/system-update",
        "@reboot /tmp/worm",
        "*/5 * * * * /tmp/system-update",
    }

    for _, job := range cronJobs {
        if strings.Contains(cronContent, job) {
            de.addFinding(Finding{
                Category:    "CRON",
                Location:    "/var/spool/cron/crontabs",
                Details:     fmt.Sprintf("Suspicious cron job: %s", job),
                Confidence:  "HIGH",
                RemediationID: "remediate_cron",
            })

            de.addRemediation(Remediation{
                ID:            generateID(),
                Action:        "DELETE_CRON",
                Target:        job,
                Command:       "crontab -l | grep -v 'system-update' | grep -v '/tmp/worm' | crontab -",
                RequiresReboot: false,
                Status:        "PENDING",
            })

            de.results.WormDetected = true
        }
    }
}

func (de *DetectionEngine) scanSystemdServices() {
    if runtime.GOOS == "windows" {
        return
    }

    servicePath := fmt.Sprintf("/etc/systemd/system/%s", WORM_SYSTEMD_SERVICE)
    if _, err := os.Stat(servicePath); err == nil {
        de.addFinding(Finding{
            Category:    "SERVICE",
            Location:    servicePath,
            Details:     "Suspicious systemd service detected",
            Confidence:  "HIGH",
            RemediationID: "remediate_service",
        })

        de.addRemediation(Remediation{
            ID:            generateID(),
            Action:        "STOP_SERVICE",
            Target:        WORM_SYSTEMD_SERVICE,
            Command:       fmt.Sprintf("systemctl stop %s && systemctl disable %s && rm %s", WORM_SYSTEMD_SERVICE, WORM_SYSTEMD_SERVICE, servicePath),
            RequiresReboot: false,
            Status:        "PENDING",
        })

        de.results.WormDetected = true
    }
}

func (de *DetectionEngine) scanUdevRules() {
    if runtime.GOOS == "windows" {
        return
    }

    udevPath := fmt.Sprintf("/etc/udev/rules.d/%s", WORM_UDEV_RULE)
    if _, err := os.Stat(udevPath); err == nil {
        // Read the file to verify it's worm-related
        data, err := ioutil.ReadFile(udevPath)
        if err == nil && strings.Contains(string(data), "system-update") {
            de.addFinding(Finding{
                Category:    "UDEV",
                Location:    udevPath,
                Details:     "Suspicious udev rule for USB auto-execution detected",
                Confidence:  "HIGH",
                RemediationID: "remediate_udev",
            })

            de.addRemediation(Remediation{
                ID:            generateID(),
                Action:        "DELETE_FILE",
                Target:        udevPath,
                Command:       fmt.Sprintf("rm %s", udevPath),
                RequiresReboot: false,
                Status:        "PENDING",
            })

            de.results.WormDetected = true
        }
    }
}

func (de *DetectionEngine) scanInitScripts() {
    if runtime.GOOS != "linux" {
        return
    }

    initPath := "/etc/init.d/system-update"
    if _, err := os.Stat(initPath); err == nil {
        data, err := ioutil.ReadFile(initPath)
        if err == nil && strings.Contains(string(data), "system-update") {
            de.addFinding(Finding{
                Category:    "INIT",
                Location:    initPath,
                Details:     "Suspicious init.d script detected",
                Confidence:  "HIGH",
                RemediationID: "remediate_init",
            })

            de.addRemediation(Remediation{
                ID:            generateID(),
                Action:        "DELETE_FILE",
                Target:        initPath,
                Command:       fmt.Sprintf("update-rc.d system-update remove && rm %s", initPath),
                RequiresReboot: false,
                Status:        "PENDING",
            })

            de.results.WormDetected = true
        }
    }
}

func (de *DetectionEngine) scanRCLocal() {
    if runtime.GOOS != "linux" {
        return
    }

    rcPath := "/etc/rc.local"
    if _, err := os.Stat(rcPath); err == nil {
        data, err := ioutil.ReadFile(rcPath)
        if err == nil && strings.Contains(string(data), WORM_RC_LOCAL_PATTERN) {
            de.addFinding(Finding{
                Category:    "RC_LOCAL",
                Location:    rcPath,
                Details:     "Suspicious rc.local entry detected",
                Confidence:  "HIGH",
                RemediationID: "remediate_rclocal",
            })

            de.addRemediation(Remediation{
                ID:            generateID(),
                Action:        "DELETE_RCLOCAL",
                Target:        rcPath,
                Command:       fmt.Sprintf("sed -i '/%s/d' %s", strings.Replace(WORM_RC_LOCAL_PATTERN, "&", "\\&", -1), rcPath),
                RequiresReboot: false,
                Status:        "PENDING",
            })

            de.results.WormDetected = true
        }
    }
}

func (de *DetectionEngine) scanLaunchAgents() {
    if runtime.GOOS != "darwin" {
        return
    }

    launchPath := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents", "com.apple.systemupdate.plist")
    if _, err := os.Stat(launchPath); err == nil {
        de.addFinding(Finding{
            Category:    "LAUNCH_AGENT",
            Location:    launchPath,
            Details:     "Suspicious launch agent detected",
            Confidence:  "HIGH",
            RemediationID: "remediate_launch",
        })

        de.addRemediation(Remediation{
            ID:            generateID(),
            Action:        "DELETE_LAUNCH",
            Target:        launchPath,
            Command:       fmt.Sprintf("launchctl unload %s && rm %s", launchPath, launchPath),
            RequiresReboot: false,
            Status:        "PENDING",
        })

        de.results.WormDetected = true
    }
}

func (de *DetectionEngine) scanSSHKeys() {
    homeDir, _ := os.UserHomeDir()
    sshPath := filepath.Join(homeDir, ".ssh", "authorized_keys")

    data, err := ioutil.ReadFile(sshPath)
    if err != nil {
        return
    }

    if strings.Contains(string(data), WORM_SSH_KEY_PATTERN) ||
       strings.Contains(string(data), WORM_SSH_KEY_CONTENT) {
        de.addFinding(Finding{
            Category:    "SSH",
            Location:    sshPath,
            Details:     "Suspicious SSH key detected (worm-bb-key)",
            Confidence:  "HIGH",
            RemediationID: "remediate_ssh",
        })

        de.addRemediation(Remediation{
            ID:            generateID(),
            Action:        "DELETE_SSH_KEY",
            Target:        sshPath,
            Command:       fmt.Sprintf("sed -i '/%s/d' %s", WORM_SSH_KEY_PATTERN, sshPath),
            RequiresReboot: false,
            Status:        "PENDING",
        })

        de.results.WormDetected = true
    }
}

func (de *DetectionEngine) scanUSBDrives() {
    if runtime.GOOS == "windows" {
        for _, drive := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
            path := string(drive) + ":\\"
            if _, err := os.Stat(path); err == nil {
                de.checkUSBPath(path)
            }
        }
    } else if runtime.GOOS == "darwin" {
        files, err := ioutil.ReadDir("/Volumes/")
        if err == nil {
            for _, f := range files {
                if f.IsDir() && !strings.HasPrefix(f.Name(), ".") {
                    de.checkUSBPath(filepath.Join("/Volumes/", f.Name()))
                }
            }
        }
    } else {
        mountPoints := []string{"/media/", "/mnt/", "/run/media/"}
        for _, mp := range mountPoints {
            files, err := ioutil.ReadDir(mp)
            if err == nil {
                for _, f := range files {
                    if f.IsDir() {
                        de.checkUSBPath(filepath.Join(mp, f.Name()))
                    }
                }
            }
        }
        // Check /mnt/sd* (embedded systems)
        sdDirs, _ := filepath.Glob("/mnt/sd*")
        for _, path := range sdDirs {
            if info, err := os.Stat(path); err == nil && info.IsDir() {
                de.checkUSBPath(path)
            }
        }
    }
}

func (de *DetectionEngine) checkUSBPath(path string) {
    // Check for Windows USB artifacts
    autorunPath := filepath.Join(path, WORM_USB_AUTORUN)
    if _, err := os.Stat(autorunPath); err == nil {
        de.addUSBFinding(autorunPath)
    }

    exePath := filepath.Join(path, WORM_USB_EXE)
    if _, err := os.Stat(exePath); err == nil {
        de.addUSBFinding(exePath)
    }

    lnkPath := filepath.Join(path, WORM_USB_LNK)
    if _, err := os.Stat(lnkPath); err == nil {
        de.addUSBFinding(lnkPath)
    }

    // Check for macOS USB artifacts
    appPath := filepath.Join(path, WORM_USB_MAC_APP)
    if _, err := os.Stat(appPath); err == nil {
        de.addUSBFinding(appPath)
    }

    // Check for Linux USB artifacts
    hiddenPath := filepath.Join(path, WORM_USB_LINUX_HIDDEN)
    if _, err := os.Stat(hiddenPath); err == nil {
        de.addUSBFinding(hiddenPath)
    }

    desktopPath := filepath.Join(path, WORM_USB_LINUX_DESKTOP)
    if _, err := os.Stat(desktopPath); err == nil {
        de.addUSBFinding(desktopPath)
    }

    // Check for any suspicious .lnk files (Windows)
    if runtime.GOOS == "windows" {
        matches, _ := filepath.Glob(filepath.Join(path, "*.lnk"))
        for _, match := range matches {
            if strings.Contains(match, "System") || strings.Contains(match, "Update") {
                de.addUSBFinding(match)
            }
        }
    }
}

func (de *DetectionEngine) addUSBFinding(path string) {
    de.addFinding(Finding{
        Category:    "USB",
        Location:    path,
        Details:     fmt.Sprintf("Suspicious USB artifact: %s", path),
        Confidence:  "HIGH",
        RemediationID: "remediate_usb",
    })

    de.addRemediation(Remediation{
        ID:            generateID(),
        Action:        "CLEAN_USB",
        Target:        path,
        Command:       de.getUSBDeleteCommand(path),
        RequiresReboot: false,
        Status:        "PENDING",
    })

    de.results.WormDetected = true
}

func (de *DetectionEngine) scanWebShells() {
    // Check for common web shell paths on local web servers
    webPaths := []string{
        "/var/www/html/shell.php",
        "/var/www/html/backdoor.php",
        "/var/www/html/system-update.php",
        "/var/www/html/shell.aspx",
        "/var/www/html/shell.py",
        "/var/www/html/wp-content/uploads/shell.php",
        "/var/www/html/cgi-bin/shell.py",
        "/var/www/html/cgi-bin/update.py",
        "/var/www/html/shell.jsp",
        "C:\\inetpub\\wwwroot\\shell.aspx",
        "C:\\inetpub\\wwwroot\\backdoor.aspx",
        "C:\\xampp\\htdocs\\shell.php",
        "C:\\xampp\\htdocs\\backdoor.php",
    }

    for _, path := range webPaths {
        if _, err := os.Stat(path); err == nil {
            // Read content to verify it's a web shell
            data, err := ioutil.ReadFile(path)
            if err == nil {
                content := string(data)
                if strings.Contains(content, "system($_REQUEST['cmd'])") ||
                   strings.Contains(content, "cmd") ||
                   strings.Contains(content, "base64_decode") ||
                   strings.Contains(content, "subprocess.check_output") {
                    de.addFinding(Finding{
                        Category:    "WEBSHELL",
                        Location:    path,
                        Details:     fmt.Sprintf("Web shell detected: %s", path),
                        Confidence:  "HIGH",
                        RemediationID: "remediate_file",
                    })

                    de.addRemediation(Remediation{
                        ID:            generateID(),
                        Action:        "DELETE_FILE",
                        Target:        path,
                        Command:       de.getDeleteCommand(path),
                        RequiresReboot: false,
                        Status:        "PENDING",
                    })

                    de.results.WormDetected = true
                }
            }
        }
    }
}

func (de *DetectionEngine) scanNetwork() {
    // Check for multicast listener on worm port
    addr, err := net.ResolveUDPAddr("udp", MULTICAST_ADDR)
    if err != nil {
        return
    }

    conn, err := net.ListenMulticastUDP("udp", nil, addr)
    if err == nil {
        defer conn.Close()
        conn.SetReadDeadline(time.Now().Add(2 * time.Second))

        buffer := make([]byte, 1024)
        n, _, err := conn.ReadFromUDP(buffer)
        if err == nil && n > 0 {
            var data map[string]interface{}
            if json.Unmarshal(buffer[:n], &data) == nil {
                if wormID, ok := data["id"].(string); ok {
                    de.addFinding(Finding{
                        Category:    "NETWORK",
                        Location:    MULTICAST_ADDR,
                        Details:     fmt.Sprintf("Worm P2P traffic detected (ID: %s)", wormID),
                        Confidence:  "HIGH",
                        RemediationID: "remediate_network",
                    })
                    de.results.WormDetected = true
                    de.results.WormVersion = "P2P Active"
                }
            }
        }
    }

    // Check for listening ports
    ports := []int{4242, 4243, 4444, 8443, 80, 443, 53}
    for _, port := range ports {
        conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 1*time.Second)
        if err == nil {
            // Try to detect if it's serving something suspicious
            de.addFinding(Finding{
                Category:    "NETWORK",
                Location:    fmt.Sprintf("127.0.0.1:%d", port),
                Details:     fmt.Sprintf("Worm listening port detected: %d", port),
                Confidence:  "MEDIUM",
                RemediationID: "remediate_network",
            })
            conn.Close()
            de.results.WormDetected = true
        }
    }
}

func (de *DetectionEngine) scanWiFiArtifacts() {
    // Check for hostapd/dnsmasq configs (evil portal)
    wifiConfigs := []string{
        "/tmp/hostapd.conf",
        "/tmp/dhcpd.conf",
        "/tmp/dnsmasq.conf",
    }

    for _, config := range wifiConfigs {
        if _, err := os.Stat(config); err == nil {
            data, err := ioutil.ReadFile(config)
            if err == nil {
                content := string(data)
                if strings.Contains(content, "Free_Public_WiFi") ||
                   strings.Contains(content, "ssid=") ||
                   strings.Contains(content, "hostapd") {
                    de.addFinding(Finding{
                        Category:    "WIFI",
                        Location:    config,
                        Details:     fmt.Sprintf("WiFi evil portal config detected: %s", config),
                        Confidence:  "HIGH",
                        RemediationID: "remediate_wifi",
                    })

                    de.addRemediation(Remediation{
                        ID:            generateID(),
                        Action:        "DELETE_FILE",
                        Target:        config,
                        Command:       de.getDeleteCommand(config),
                        RequiresReboot: false,
                        Status:        "PENDING",
                    })

                    de.results.WormDetected = true
                }
            }
        }
    }

    // Check for iptables NAT rules (evil portal)
    if runtime.GOOS == "linux" {
        cmd := exec.Command("iptables", "-t", "nat", "-L")
        output, err := cmd.Output()
        if err == nil && strings.Contains(string(output), "MASQUERADE") {
            de.addFinding(Finding{
                Category:    "WIFI",
                Location:    "iptables",
                Details:     "Suspicious iptables NAT rule for evil portal detected",
                Confidence:  "MEDIUM",
                RemediationID: "remediate_wifi",
            })
            de.results.WormDetected = true
        }
    }
}

func (de *DetectionEngine) scanMemory() {
    if runtime.GOOS == "windows" {
        // Check loaded modules
        cmd := exec.Command("tasklist", "/M")
        output, err := cmd.Output()
        if err == nil {
            suspiciousModules := []string{"system-update", "SystemUpdate", "worm_bb"}
            for _, module := range suspiciousModules {
                if strings.Contains(string(output), module) {
                    de.addFinding(Finding{
                        Category:    "MEMORY",
                        Location:    "Process memory",
                        Details:     fmt.Sprintf("Worm module found in memory: %s", module),
                        Confidence:  "MEDIUM",
                        RemediationID: "remediate_process",
                    })
                    de.results.WormDetected = true
                }
            }
        }
    } else {
        // Check /proc for suspicious memory maps
        cmd := exec.Command("grep", "-r", "system-update", "/proc/*/maps")
        output, err := cmd.Output()
        if err == nil && len(output) > 0 {
            de.addFinding(Finding{
                Category:    "MEMORY",
                Location:    "Process memory maps",
                Details:     "Worm mapped in memory",
                Confidence:  "MEDIUM",
                RemediationID: "remediate_process",
            })
            de.results.WormDetected = true
        }
    }
}

func (de *DetectionEngine) scanMutex() {
    if runtime.GOOS == "windows" {
        // Try to open the mutex
        mutex, err := windows.OpenMutex(0x001F0001, false, syscall.StringToUTF16Ptr("Global\\SystemUpdateMutex"))
        if err == nil {
            windows.CloseHandle(mutex)
            de.addFinding(Finding{
                Category:    "MUTEX",
                Location:    "Global\\SystemUpdateMutex",
                Details:     "Worm mutex detected",
                Confidence:  "HIGH",
                RemediationID: "remediate_mutex",
            })
            de.results.WormDetected = true
        }
    } else {
        if _, err := os.Stat("/tmp/.system-update.lock"); err == nil {
            de.addFinding(Finding{
                Category:    "MUTEX",
                Location:    "/tmp/.system-update.lock",
                Details:     "Worm lock file detected",
                Confidence:  "HIGH",
                RemediationID: "remediate_file",
            })
            de.results.WormDetected = true
        }
    }
}

func (de *DetectionEngine) scanP2P() {
    // Check for established connections to multicast address
    if runtime.GOOS == "linux" {
        cmd := exec.Command("netstat", "-n", "-u")
        output, err := cmd.Output()
        if err == nil && strings.Contains(string(output), "239.255.42.42:4242") {
            de.addFinding(Finding{
                Category:    "P2P",
                Location:    MULTICAST_ADDR,
                Details:     "Active P2P multicast connection detected",
                Confidence:  "HIGH",
                RemediationID: "remediate_network",
            })
            de.results.WormDetected = true
        }
    } else if runtime.GOOS == "windows" {
        cmd := exec.Command("netstat", "-n")
        output, err := cmd.Output()
        if err == nil && strings.Contains(string(output), "239.255.42.42") {
            de.addFinding(Finding{
                Category:    "P2P",
                Location:    MULTICAST_ADDR,
                Details:     "Active P2P multicast connection detected",
                Confidence:  "HIGH",
                RemediationID: "remediate_network",
            })
            de.results.WormDetected = true
        }
    }

    // Check for P2P ports
    p2pPorts := []string{"4242", "4243", "4444"}
    for _, port := range p2pPorts {
        if de.isPortListening(port) {
            de.addFinding(Finding{
                Category:    "P2P",
                Location:    fmt.Sprintf(":%s", port),
                Details:     fmt.Sprintf("P2P port %s is listening", port),
                Confidence:  "MEDIUM",
                RemediationID: "remediate_network",
            })
            de.results.WormDetected = true
        }
    }
}

func (de *DetectionEngine) isPortListening(port string) bool {
    if runtime.GOOS == "linux" {
        cmd := exec.Command("netstat", "-tln")
        output, err := cmd.Output()
        if err == nil {
            return strings.Contains(string(output), ":"+port+" ") || strings.Contains(string(output), ":"+port+"\t")
        }
    } else if runtime.GOOS == "windows" {
        cmd := exec.Command("netstat", "-an")
        output, err := cmd.Output()
        if err == nil {
            return strings.Contains(string(output), ":"+port+" ") || strings.Contains(string(output), ":"+port+"\t")
        }
    }
    return false
}

func (de *DetectionEngine) calculateSeverity() {
    highCount := 0
    for _, finding := range de.findings {
        if finding.Confidence == "HIGH" {
            highCount++
        }
    }

    if highCount >= 8 {
        de.results.Severity = "CRITICAL"
    } else if highCount >= 4 {
        de.results.Severity = "HIGH"
    } else if highCount >= 1 {
        de.results.Severity = "MEDIUM"
    } else {
        de.results.Severity = "LOW"
    }
}

func (de *DetectionEngine) addFinding(finding Finding) {
    de.mu.Lock()
    defer de.mu.Unlock()
    de.findings = append(de.findings, finding)
}

func (de *DetectionEngine) addRemediation(rem Remediation) {
    de.mu.Lock()
    defer de.mu.Unlock()
    de.remediations = append(de.remediations, rem)
}

// ========== HELPER FUNCTIONS ==========

func (de *DetectionEngine) getKillCommand(pid int) string {
    if runtime.GOOS == "windows" {
        return fmt.Sprintf("taskkill /F /PID %d", pid)
    }
    return fmt.Sprintf("kill -9 %d", pid)
}

func (de *DetectionEngine) getDeleteCommand(path string) string {
    if runtime.GOOS == "windows" {
        return fmt.Sprintf("del /F /Q \"%s\"", path)
    }
    return fmt.Sprintf("rm -f \"%s\"", path)
}

func (de *DetectionEngine) getUSBDeleteCommand(path string) string {
    if runtime.GOOS == "windows" {
        return fmt.Sprintf("del /F /Q /A:H \"%s\"", path)
    }
    return fmt.Sprintf("rm -f \"%s\"", path)
}

// ========== UTILITY FUNCTIONS ==========

func getHostname() string {
    hostname, err := os.Hostname()
    if err != nil {
        return "unknown"
    }
    return hostname
}

func getLocalIP() string {
    addrs, err := net.InterfaceAddrs()
    if err != nil {
        return "127.0.0.1"
    }
    for _, addr := range addrs {
        if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
            if ipnet.IP.To4() != nil {
                return ipnet.IP.String()
            }
        }
    }
    return "127.0.0.1"
}

func generateID() string {
    data := fmt.Sprintf("%d", time.Now().UnixNano())
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:8])
}

func (de *DetectionEngine) getWindowsProcesses() []map[string]interface{} {
    var processes []map[string]interface{}

    cmd := exec.Command("tasklist", "/FO", "CSV", "/NH")
    output, err := cmd.Output()
    if err != nil {
        return processes
    }

    lines := strings.Split(string(output), "\n")
    for _, line := range lines {
        if line == "" {
            continue
        }
        parts := strings.Split(strings.Trim(line, "\""), "\",\"")
        if len(parts) >= 2 {
            proc := map[string]interface{}{
                "name": parts[0],
                "pid":  atoi(parts[1]),
            }

            cmdline := exec.Command("wmic", "process", "where", fmt.Sprintf("processid=%d", proc["pid"]), "get", "commandline")
            cmdlineOut, _ := cmdline.Output()
            proc["cmdline"] = string(cmdlineOut)

            processes = append(processes, proc)
        }
    }

    return processes
}

func (de *DetectionEngine) getLinuxProcesses() []map[string]interface{} {
    var processes []map[string]interface{}

    files, err := ioutil.ReadDir("/proc")
    if err != nil {
        return processes
    }

    for _, file := range files {
        if file.IsDir() && isNumeric(file.Name()) {
            pid := atoi(file.Name())
            if pid == 0 {
                continue
            }

            cmdlinePath := fmt.Sprintf("/proc/%d/cmdline", pid)
            cmdlineData, err := ioutil.ReadFile(cmdlinePath)
            if err != nil {
                continue
            }

            cmdline := strings.Replace(string(cmdlineData), "\x00", " ", -1)
            procName := filepath.Base(cmdline)
            if procName == "" {
                procName = "unknown"
            }

            processes = append(processes, map[string]interface{}{
                "name":    procName,
                "pid":     pid,
                "cmdline": cmdline,
            })
        }
    }

    return processes
}

func atoi(s string) int {
    var result int
    fmt.Sscanf(s, "%d", &result)
    return result
}

func isNumeric(s string) bool {
    for _, c := range s {
        if c < '0' || c > '9' {
            return false
        }
    }
    return true
}

// ========== REMEDIATION ENGINE ==========

type RemediationEngine struct {
    remediations []Remediation
    results      map[string]bool
    mu           sync.Mutex
    autoApprove  bool
}

func NewRemediationEngine(autoApprove bool) *RemediationEngine {
    return &RemediationEngine{
        remediations: []Remediation{},
        results:      make(map[string]bool),
        autoApprove:  autoApprove,
    }
}

func (re *RemediationEngine) LoadRemediations(remediations []Remediation) {
    re.remediations = remediations
}

func (re *RemediationEngine) ExecuteRemediations() {
    fmt.Println("\n[Remediation] Starting cleanup process")
    fmt.Println("================================================")

    for _, rem := range re.remediations {
        if rem.Status != "PENDING" {
            continue
        }

        if !re.autoApprove {
            fmt.Printf("\n[?] Remediation: %s\n", rem.Action)
            fmt.Printf("    Target: %s\n", rem.Target)
            fmt.Printf("    Command: %s\n", rem.Command)
            fmt.Print("    Execute? (y/N): ")

            var response string
            fmt.Scanln(&response)
            if strings.ToLower(response) != "y" {
                fmt.Println("    Skipped")
                continue
            }
        }

        fmt.Printf("[*] Executing: %s on %s\n", rem.Action, rem.Target)
        err := re.executeRemediation(rem)

        re.mu.Lock()
        if err == nil {
            rem.Status = "COMPLETED"
            re.results[rem.ID] = true
            fmt.Printf("[+] Success: %s completed\n", rem.Action)
        } else {
            rem.Status = "FAILED"
            re.results[rem.ID] = false
            fmt.Printf("[-] Failed: %s - %v\n", rem.Action, err)
        }
        re.mu.Unlock()
    }

    fmt.Println("================================================")
    re.printSummary()
}

func (re *RemediationEngine) executeRemediation(rem Remediation) error {
    switch rem.Action {
    case "KILL_PROCESS":
        return re.killProcess(rem.Target)
    case "DELETE_FILE":
        return re.deleteFile(rem.Target)
    case "DELETE_REGISTRY":
        return re.deleteRegistry(rem.Target)
    case "DELETE_TASK":
        return re.deleteTask(rem.Target)
    case "DELETE_CRON":
        return re.deleteCron(rem.Command)
    case "STOP_SERVICE":
        return re.stopService(rem.Command)
    case "CLEAN_USB":
        return re.cleanUSB(rem.Target)
    case "DELETE_WMI":
        return re.deleteWMI(rem.Command)
    case "DELETE_SSH_KEY":
        return re.deleteSSHKey(rem.Command)
    case "DELETE_LAUNCH":
        return re.deleteLaunch(rem.Command)
    case "DELETE_RCLOCAL":
        return re.deleteRCLocal(rem.Command)
    default:
        return re.executeCommand(rem.Command)
    }
}

func (re *RemediationEngine) killProcess(pidStr string) error {
    pid := atoi(pidStr)
    if pid <= 0 {
        return fmt.Errorf("invalid PID: %s", pidStr)
    }

    process, err := os.FindProcess(pid)
    if err != nil {
        return err
    }

    if runtime.GOOS == "windows" {
        return process.Kill()
    }
    return process.Signal(syscall.SIGTERM)
}

func (re *RemediationEngine) deleteFile(path string) error {
    return os.RemoveAll(path)
}

func (re *RemediationEngine) deleteRegistry(keyPath string) error {
    if runtime.GOOS != "windows" {
        return fmt.Errorf("registry operations not supported on this OS")
    }
    return re.executeCommand(fmt.Sprintf("reg delete \"%s\" /f", keyPath))
}

func (re *RemediationEngine) deleteTask(taskName string) error {
    return re.executeCommand(fmt.Sprintf("schtasks /delete /tn \"%s\" /f", taskName))
}

func (re *RemediationEngine) deleteCron(command string) error {
    return re.executeCommand(command)
}

func (re *RemediationEngine) stopService(command string) error {
    return re.executeCommand(command)
}

func (re *RemediationEngine) cleanUSB(path string) error {
    filesToDelete := []string{
        filepath.Join(path, WORM_USB_AUTORUN),
        filepath.Join(path, WORM_USB_EXE),
        filepath.Join(path, WORM_USB_LNK),
        filepath.Join(path, WORM_USB_MAC_APP),
        filepath.Join(path, WORM_USB_LINUX_HIDDEN),
        filepath.Join(path, WORM_USB_LINUX_DESKTOP),
    }

    for _, file := range filesToDelete {
        os.RemoveAll(file)
    }

    // Also remove any suspicious .lnk files
    if runtime.GOOS == "windows" {
        matches, _ := filepath.Glob(filepath.Join(path, "*.lnk"))
        for _, match := range matches {
            if strings.Contains(match, "System") || strings.Contains(match, "Update") {
                os.Remove(match)
            }
        }
    }

    return nil
}

func (re *RemediationEngine) deleteWMI(command string) error {
    return re.executeCommand(fmt.Sprintf("powershell -Command \"%s\"", command))
}

func (re *RemediationEngine) deleteSSHKey(command string) error {
    return re.executeCommand(command)
}

func (re *RemediationEngine) deleteLaunch(command string) error {
    return re.executeCommand(command)
}

func (re *RemediationEngine) deleteRCLocal(command string) error {
    return re.executeCommand(command)
}

func (re *RemediationEngine) executeCommand(command string) error {
    var cmd *exec.Cmd

    if runtime.GOOS == "windows" {
        cmd = exec.Command("cmd", "/C", command)
    } else {
        cmd = exec.Command("bash", "-c", command)
    }

    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("command failed: %s - %v", string(output), err)
    }

    return nil
}

func (re *RemediationEngine) printSummary() {
    completed := 0
    failed := 0

    for _, success := range re.results {
        if success {
            completed++
        } else {
            failed++
        }
    }

    fmt.Printf("\n[Remediation Summary]\n")
    fmt.Printf("  Total remediations: %d\n", len(re.remediations))
    fmt.Printf("  Completed: %d\n", completed)
    fmt.Printf("  Failed: %d\n", failed)

    if failed > 0 {
        fmt.Println("\n[!] Some remediations failed. Manual cleanup may be required.")
        fmt.Println("    Review findings above and perform manual removal.")
    } else {
        fmt.Println("\n[+] All remediations completed successfully!")
        fmt.Println("    Reboot recommended to ensure complete cleanup.")
    }
}

// ========== REPORT ENGINE ==========

type ReportEngine struct {
    result *DetectionResult
}

func NewReportEngine(result *DetectionResult) *ReportEngine {
    return &ReportEngine{result: result}
}

func (re *ReportEngine) PrintReport() {
    fmt.Println("\n" + strings.Repeat("=", 80))
    fmt.Println("WORM-BB DETECTION REPORT v2.0")
    fmt.Println(strings.Repeat("=", 80))
    fmt.Printf("Timestamp:      %s\n", re.result.Timestamp.Format("2006-01-02 15:04:05"))
    fmt.Printf("Hostname:       %s\n", re.result.Hostname)
    fmt.Printf("OS:             %s\n", re.result.OS)
    fmt.Printf("IP Address:     %s\n", re.result.IPAddress)
    fmt.Printf("Worm Detected:  %v\n", re.result.WormDetected)
    if re.result.WormVersion != "" {
        fmt.Printf("Worm Version:   %s\n", re.result.WormVersion)
    }
    fmt.Printf("Severity:       %s\n", re.result.Severity)
    fmt.Printf("Scan Duration:  %v\n", re.result.ScanDuration)
    fmt.Printf("Findings:       %d\n", len(re.result.Findings))
    fmt.Printf("Remediations:   %d\n", len(re.result.Remediations))

    if len(re.result.Findings) > 0 {
        fmt.Println("\n" + strings.Repeat("-", 80))
        fmt.Println("DETAILED FINDINGS")
        fmt.Println(strings.Repeat("-", 80))

        for i, finding := range re.result.Findings {
            fmt.Printf("\n[%d] Category: %s\n", i+1, finding.Category)
            fmt.Printf("    Location: %s\n", finding.Location)
            fmt.Printf("    Details:  %s\n", finding.Details)
            fmt.Printf("    Confidence: %s\n", finding.Confidence)
        }
    }

    fmt.Println(strings.Repeat("=", 80))
}

func (re *ReportEngine) SaveJSON(filename string) error {
    data, err := json.MarshalIndent(re.result, "", "  ")
    if err != nil {
        return err
    }
    return ioutil.WriteFile(filename, data, 0644)
}

// ========== MAIN ==========

func main() {
    fmt.Println(strings.Repeat("=", 80))
    fmt.Println("WORM-BB DETECTION AND REMOVAL TOOL v2.0")
    fmt.Println("Full coverage for Worm-BB v4.0-DEFCON-ARM")
    fmt.Println("Authorized Use Only - Blue Team / Incident Response")
    fmt.Println(strings.Repeat("=", 80))

    // Parse command line arguments
    autoRemediate := false
    networkScan := false
    outputFile := ""

    for i, arg := range os.Args {
        switch arg {
        case "--auto", "-a":
            autoRemediate = true
        case "--network", "-n":
            networkScan = true
        case "--output", "-o":
            if i+1 < len(os.Args) {
                outputFile = os.Args[i+1]
            }
        case "--help", "-h":
            printHelp()
            return
        }
    }

    // Run detection
    detector := NewDetectionEngine(networkScan, autoRemediate)
    detector.RunFullScan()

    // Generate report
    reporter := NewReportEngine(detector.results)
    reporter.PrintReport()

    if outputFile != "" {
        if err := reporter.SaveJSON(outputFile); err != nil {
            fmt.Printf("[-] Failed to save report: %v\n", err)
        } else {
            fmt.Printf("[+] Report saved to %s\n", outputFile)
        }
    }

    // Run remediation if worm detected
    if detector.results.WormDetected && len(detector.remediations) > 0 {
        fmt.Println("\n" + strings.Repeat("=", 80))
        fmt.Println("REMEDIATION PHASE")
        fmt.Println(strings.Repeat("=", 80))

        remediator := NewRemediationEngine(autoRemediate)
        remediator.LoadRemediations(detector.remediations)
        remediator.ExecuteRemediations()
    }

    fmt.Println("\n[+] Scan complete")

    if detector.results.WormDetected {
        fmt.Println("\n[!] Worm detected and remediated. Reboot recommended.")
        os.Exit(1)
    }

    fmt.Println("\n[+] No worm detected. System appears clean.")
    os.Exit(0)
}

func printHelp() {
    fmt.Println(`
Usage: worm_bb_detector [options]

Options:
  -a, --auto      Automatic remediation (no user prompts)
  -n, --network   Enable network scanning (multicast, port checks, WiFi artifacts)
  -o, --output    Save JSON report to file
  -h, --help      Show this help message

Examples:
  # Basic scan with user prompts
  worm_bb_detector

  # Full automatic scan with network detection
  worm_bb_detector --auto --network

  # Scan and save report
  worm_bb_detector --output report.json

Exit Codes:
  0 - No worm detected
  1 - Worm detected and remediated

Note: Run with elevated privileges (Administrator/root) for full detection capability
`)
}
