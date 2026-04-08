// worm_bb_detector.go - Worm-BB Detection and Removal Tool
// RED TEAM / BLUE TEAM UTILITY - Authorized Use Only

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
    "syscall"
    "time"
    
    "golang.org/x/sys/windows"
    "golang.org/x/sys/windows/registry"
)

// ========== CONSTANTS ==========

const (
    VERSION = "1.0"
    MUTEX_NAME = "Global\\SystemUpdateMutex"
    MULTICAST_ADDR = "239.255.42.42:4242"
    LOCK_FILE = "/tmp/.system-update.lock"
    WORM_PATHS_WINDOWS = "C:\\Windows\\Temp\\system-update.exe,%APPDATA%\\Microsoft\\Windows\\Start Menu\\Programs\\Startup\\SystemUpdate.exe,%TEMP%\\worm*.exe"
    WORM_PATHS_LINUX = "/tmp/system-update,/tmp/.system-update,/etc/systemd/system/system-update.service,/tmp/.system-update.lock"
    WORM_REGISTRY_KEYS = "HKCU\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Run\\SystemUpdate,HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Run\\SystemUpdate"
    WORM_SCHEDULED_TASKS = "SystemUpdateTask,SystemUpdateTask_startup"
    WORM_CRON_JOBS = "@reboot /tmp/system-update,*/30 * * * * /tmp/system-update"
    WORM_SYSTEMD_SERVICE = "system-update.service"
    WORM_WMI_FILTER = "SystemUpdateFilter"
    WORM_WMI_CONSUMER = "SystemUpdateConsumer"
    WORM_USB_AUTORUN = "autorun.inf"
    WORM_USB_EXE = "SystemUpdate.exe"
    WORM_SSH_KEY_PATTERN = "worm-bb-key"
    WORM_UDEV_RULE = "99-usb-autorun.rules"
)

// ========== DATA STRUCTURES ==========

type DetectionResult struct {
    Timestamp      time.Time              `json:"timestamp"`
    Hostname       string                 `json:"hostname"`
    OS             string                 `json:"os"`
    IPAddress      string                 `json:"ip_address"`
    WormDetected   bool                   `json:"worm_detected"`
    Severity       string                 `json:"severity"` // CRITICAL, HIGH, MEDIUM, LOW
    Findings       []Finding              `json:"findings"`
    Remediations   []Remediation          `json:"remediations"`
    ScanDuration   time.Duration          `json:"scan_duration"`
}

type Finding struct {
    Category     string   `json:"category"`     // PROCESS, FILE, REGISTRY, SCHEDULED_TASK, CRON, SERVICE, NETWORK, USB, WMI, SSH
    Location     string   `json:"location"`
    Details      string   `json:"details"`
    Confidence   string   `json:"confidence"`   // HIGH, MEDIUM, LOW
    RemediationID string  `json:"remediation_id"`
}

type Remediation struct {
    ID           string   `json:"id"`
    Action       string   `json:"action"`       // KILL_PROCESS, DELETE_FILE, DELETE_REGISTRY, DELETE_TASK, DELETE_CRON, STOP_SERVICE, BLOCK_NETWORK, CLEAN_USB
    Target       string   `json:"target"`
    Command      string   `json:"command"`
    RequiresReboot bool   `json:"requires_reboot"`
    Status       string   `json:"status"`       // PENDING, COMPLETED, FAILED
}

type WormSignature struct {
    Name        string   `json:"name"`
    Pattern     string   `json:"pattern"`
    Type        string   `json:"type"` // FILENAME, HASH, REGEX, PE_IMPORT, STRING
    Severity    string   `json:"severity"`
    Hashes      []string `json:"hashes,omitempty"`
}

// ========== WORM SIGNATURES ==========

var wormSignatures = []WormSignature{
    {
        Name:     "Worm-BB Process Name",
        Pattern:  `(?i)(system-update|SystemUpdate|worm_bb|worm-bb)`,
        Type:     "REGEX",
        Severity: "HIGH",
    },
    {
        Name:     "Worm-BB Mutex",
        Pattern:  "Global\\\\SystemUpdateMutex",
        Type:     "STRING",
        Severity: "HIGH",
    },
    {
        Name:     "Worm-BB Multicast Communication",
        Pattern:  "239.255.42.42",
        Type:     "STRING",
        Severity: "HIGH",
    },
    {
        Name:     "Worm-BB File Name - Linux",
        Pattern:  "/tmp/system-update",
        Type:     "STRING",
        Severity: "HIGH",
    },
    {
        Name:     "Worm-BB Service Name",
        Pattern:  "system-update.service",
        Type:     "STRING",
        Severity: "MEDIUM",
    },
    {
        Name:     "Worm-BB Registry Key",
        Pattern:  "SystemUpdate",
        Type:     "STRING",
        Severity: "MEDIUM",
    },
}

// ========== DETECTION ENGINE ==========

type DetectionEngine struct {
    results       *DetectionResult
    findings      []Finding
    remediations  []Remediation
    mu            sync.Mutex
    wormHashes    map[string]bool
    networkScan   bool
}

func NewDetectionEngine(networkScan bool) *DetectionEngine {
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
    }
}

func (de *DetectionEngine) RunFullScan() {
    startTime := time.Now()
    defer func() {
        de.results.ScanDuration = time.Since(startTime)
        de.results.Findings = de.findings
        de.results.Remediations = de.remediations
    }()
    
    fmt.Println("[Worm-BB Detector] Starting comprehensive scan")
    fmt.Println("================================================")
    
    // Process scanning
    fmt.Println("[*] Scanning for worm processes...")
    de.scanProcesses()
    
    // File system scanning
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
    }
    
    // Linux-specific scans
    if runtime.GOOS == "linux" {
        fmt.Println("[*] Scanning cron jobs...")
        de.scanCronJobs()
        
        fmt.Println("[*] Scanning systemd services...")
        de.scanSystemdServices()
        
        fmt.Println("[*] Scanning udev rules...")
        de.scanUdevRules()
    }
    
    // Common scans
    fmt.Println("[*] Scanning SSH authorized_keys...")
    de.scanSSHKeys()
    
    fmt.Println("[*] Scanning USB drives...")
    de.scanUSBDrives()
    
    if de.networkScan {
        fmt.Println("[*] Scanning network for worm peers...")
        de.scanNetwork()
    }
    
    fmt.Println("[*] Scanning memory for signatures...")
    de.scanMemory()
    
    fmt.Println("[*] Calculating file hashes...")
    de.calculateHashes()
    
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
        
        // Check against signatures
        for _, sig := range wormSignatures {
            if sig.Type == "REGEX" {
                matched, _ := regexp.MatchString(sig.Pattern, procName)
                if matched {
                    de.addFinding(Finding{
                        Category:    "PROCESS",
                        Location:    fmt.Sprintf("PID: %d", procPID),
                        Details:     fmt.Sprintf("Suspicious process: %s (matched signature: %s)", procName, sig.Name),
                        Confidence:  "HIGH",
                        RemediationID: "remediate_process",
                    })
                    
                    de.addRemediation(Remediation{
                        ID:            generateID(),
                        Action:        "KILL_PROCESS",
                        Target:        fmt.Sprintf("%d", procPID),
                        Command:       de.getKillCommand(procPID),
                        RequiresReboot: false,
                        Status:        "PENDING",
                    })
                    
                    de.results.WormDetected = true
                }
            }
        }
        
        // Check command line for suspicious strings
        if cmdline, ok := proc["cmdline"].(string); ok {
            if strings.Contains(cmdline, "system-update") || 
               strings.Contains(cmdline, "SystemUpdate") ||
               strings.Contains(cmdline, "worm_bb") {
                de.addFinding(Finding{
                    Category:    "PROCESS",
                    Location:    fmt.Sprintf("PID: %d", procPID),
                    Details:     fmt.Sprintf("Suspicious command line: %s", cmdline),
                    Confidence:  "HIGH",
                    RemediationID: "remediate_process",
                })
                de.results.WormDetected = true
            }
        }
    }
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
            
            // Get command line
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
            
            // Read process name
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

func (de *DetectionEngine) scanFiles() {
    var paths []string
    
    if runtime.GOOS == "windows" {
        paths = strings.Split(WORM_PATHS_WINDOWS, ",")
        // Expand environment variables
        for i, path := range paths {
            paths[i] = os.ExpandEnv(path)
        }
    } else {
        paths = strings.Split(WORM_PATHS_LINUX, ",")
    }
    
    for _, path := range paths {
        if _, err := os.Stat(path); err == nil {
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
            
            // Calculate hash for future detection
            de.hashFile(path)
        }
    }
    
    // Recursive scan of common directories
    scanDirs := []string{}
    if runtime.GOOS == "windows" {
        scanDirs = []string{os.Getenv("TEMP"), os.Getenv("APPDATA"), "C:\\Windows\\Temp"}
    } else {
        scanDirs = []string{"/tmp", "/var/tmp", "/dev/shm"}
    }
    
    for _, dir := range scanDirs {
        de.recursiveFileScan(dir)
    }
}

func (de *DetectionEngine) recursiveFileScan(dir string) {
    files, err := ioutil.ReadDir(dir)
    if err != nil {
        return
    }
    
    for _, file := range files {
        if file.IsDir() {
            // Avoid recursion depth issues
            if strings.HasPrefix(file.Name(), ".") {
                continue
            }
            de.recursiveFileScan(filepath.Join(dir, file.Name()))
        } else {
            // Check filename against patterns
            for _, sig := range wormSignatures {
                if sig.Type == "REGEX" {
                    matched, _ := regexp.MatchString(sig.Pattern, file.Name())
                    if matched {
                        fullPath := filepath.Join(dir, file.Name())
                        de.addFinding(Finding{
                            Category:    "FILE",
                            Location:    fullPath,
                            Details:     fmt.Sprintf("Suspicious filename: %s", file.Name()),
                            Confidence:  "MEDIUM",
                            RemediationID: "remediate_file",
                        })
                        de.hashFile(fullPath)
                    }
                }
            }
        }
    }
}

func (de *DetectionEngine) hashFile(path string) {
    data, err := ioutil.ReadFile(path)
    if err != nil {
        return
    }
    
    hash := sha256.Sum256(data)
    hashStr := hex.EncodeToString(hash[:])
    
    // Check against known worm hashes (would be populated from threat intel)
    de.wormHashes[hashStr] = true
}

func (de *DetectionEngine) scanRegistry() {
    if runtime.GOOS != "windows" {
        return
    }
    
    keys := strings.Split(WORM_REGISTRY_KEYS, ",")
    for _, keyPath := range keys {
        // Parse registry path
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
            value, _, err := regKey.GetStringValue(filepath.Base(key))
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
                    Command:       fmt.Sprintf("reg delete \"%s\" /v %s /f", keyPath, filepath.Base(key)),
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
    
    tasks := strings.Split(WORM_SCHEDULED_TASKS, ",")
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
}

func (de *DetectionEngine) scanCronJobs() {
    if runtime.GOOS == "windows" {
        return
    }
    
    cmd := exec.Command("crontab", "-l")
    output, err := cmd.Output()
    if err != nil {
        return
    }
    
    cronContent := string(output)
    cronJobs := strings.Split(WORM_CRON_JOBS, ",")
    
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
                Command:       "crontab -l | grep -v 'system-update' | crontab -",
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

func (de *DetectionEngine) scanSSHKeys() {
    homeDir, _ := os.UserHomeDir()
    sshPath := filepath.Join(homeDir, ".ssh", "authorized_keys")
    
    data, err := ioutil.ReadFile(sshPath)
    if err != nil {
        return
    }
    
    if strings.Contains(string(data), WORM_SSH_KEY_PATTERN) {
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
    }
}

func (de *DetectionEngine) checkUSBPath(path string) {
    // Check for autorun.inf
    autorunPath := filepath.Join(path, WORM_USB_AUTORUN)
    if _, err := os.Stat(autorunPath); err == nil {
        de.addFinding(Finding{
            Category:    "USB",
            Location:    autorunPath,
            Details:     "Suspicious autorun.inf on USB drive",
            Confidence:  "HIGH",
            RemediationID: "remediate_usb",
        })
        
        de.addRemediation(Remediation{
            ID:            generateID(),
            Action:        "CLEAN_USB",
            Target:        path,
            Command:       de.getUSBDeleteCommand(autorunPath),
            RequiresReboot: false,
            Status:        "PENDING",
        })
        
        de.results.WormDetected = true
    }
    
    // Check for worm executable
    exePath := filepath.Join(path, WORM_USB_EXE)
    if _, err := os.Stat(exePath); err == nil {
        de.addFinding(Finding{
            Category:    "USB",
            Location:    exePath,
            Details:     "Suspicious executable on USB drive",
            Confidence:  "HIGH",
            RemediationID: "remediate_usb",
        })
        
        de.addRemediation(Remediation{
            ID:            generateID(),
            Action:        "CLEAN_USB",
            Target:        path,
            Command:       de.getUSBDeleteCommand(exePath),
            RequiresReboot: false,
            Status:        "PENDING",
        })
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
            de.addFinding(Finding{
                Category:    "NETWORK",
                Location:    MULTICAST_ADDR,
                Details:     fmt.Sprintf("Worm multicast traffic detected: %s", string(buffer[:n])),
                Confidence:  "HIGH",
                RemediationID: "remediate_network",
            })
            
            de.results.WormDetected = true
        }
    }
    
    // Check for listening ports
    ports := []int{4242, 4243, 4444, 8443}
    for _, port := range ports {
        conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 1*time.Second)
        if err == nil {
            de.addFinding(Finding{
                Category:    "NETWORK",
                Location:    fmt.Sprintf("127.0.0.1:%d", port),
                Details:     fmt.Sprintf("Worm listening port detected: %d", port),
                Confidence:  "HIGH",
                RemediationID: "remediate_network",
            })
            conn.Close()
            de.results.WormDetected = true
        }
    }
}

func (de *DetectionEngine) scanMemory() {
    // This would use more advanced memory scanning techniques
    // Simplified version - check for loaded modules/dlls
    
    if runtime.GOOS == "windows" {
        cmd := exec.Command("tasklist", "/M")
        output, err := cmd.Output()
        if err == nil {
            for _, sig := range wormSignatures {
                if strings.Contains(string(output), sig.Pattern) {
                    de.addFinding(Finding{
                        Category:    "MEMORY",
                        Location:    "Process memory",
                        Details:     fmt.Sprintf("Worm signature found in memory: %s", sig.Name),
                        Confidence:  "MEDIUM",
                        RemediationID: "remediate_process",
                    })
                    de.results.WormDetected = true
                }
            }
        }
    }
}

func (de *DetectionEngine) calculateHashes() {
    // In production, would submit hashes to VirusTotal or threat intel
    for hash := range de.wormHashes {
        fmt.Printf("[*] Found suspicious hash: %s\n", hash)
    }
}

func (de *DetectionEngine) calculateSeverity() {
    highCount := 0
    for _, finding := range de.findings {
        if finding.Confidence == "HIGH" {
            highCount++
        }
    }
    
    if highCount >= 5 {
        de.results.Severity = "CRITICAL"
    } else if highCount >= 3 {
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
    // Delete worm files from USB
    filesToDelete := []string{
        filepath.Join(path, WORM_USB_AUTORUN),
        filepath.Join(path, WORM_USB_EXE),
        filepath.Join(path, "System Update.lnk"),
    }
    
    for _, file := range filesToDelete {
        os.Remove(file)
    }
    
    return nil
}

func (re *RemediationEngine) deleteWMI(command string) error {
    return re.executeCommand(fmt.Sprintf("powershell -Command \"%s\"", command))
}

func (re *RemediationEngine) deleteSSHKey(command string) error {
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

// ========== REPORTING ENGINE ==========

type ReportEngine struct {
    result *DetectionResult
}

func NewReportEngine(result *DetectionResult) *ReportEngine {
    return &ReportEngine{result: result}
}

func (re *ReportEngine) PrintReport() {
    fmt.Println("\n" + strings.Repeat("=", 80))
    fmt.Println("WORM-BB DETECTION REPORT")
    fmt.Println(strings.Repeat("=", 80))
    fmt.Printf("Timestamp:      %s\n", re.result.Timestamp.Format("2006-01-02 15:04:05"))
    fmt.Printf("Hostname:       %s\n", re.result.Hostname)
    fmt.Printf("OS:             %s\n", re.result.OS)
    fmt.Printf("IP Address:     %s\n", re.result.IPAddress)
    fmt.Printf("Worm Detected:  %v\n", re.result.WormDetected)
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

func getKillCommand(pid int) string {
    if runtime.GOOS == "windows" {
        return fmt.Sprintf("taskkill /F /PID %d", pid)
    }
    return fmt.Sprintf("kill -9 %d", pid)
}

func getDeleteCommand(path string) string {
    if runtime.GOOS == "windows" {
        return fmt.Sprintf("del /F /Q \"%s\"", path)
    }
    return fmt.Sprintf("rm -f \"%s\"", path)
}

func getUSBDeleteCommand(path string) string {
    if runtime.GOOS == "windows" {
        return fmt.Sprintf("del /F /Q /A:H \"%s\"", path)
    }
    return fmt.Sprintf("rm -f \"%s\"", path)
}

func generateID() string {
    data := fmt.Sprintf("%d", time.Now().UnixNano())
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:8])
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

// ========== MAIN ==========

func main() {
    fmt.Println(strings.Repeat("=", 80))
    fmt.Println("WORM-BB DETECTION AND REMOVAL TOOL")
    fmt.Printf("Version: %s\n", VERSION)
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
    detector := NewDetectionEngine(networkScan)
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
  -n, --network   Enable network scanning (multicast, port checks)
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
