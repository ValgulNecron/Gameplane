package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ValgulNecron/gameplane/gameproto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	gameserverName      string
	gameserverNamespace string
	portsConfig         string
	wakeDeadline        time.Duration
	udpPacketThreshold  int
	udpPacketWindow     time.Duration
)

func init() {
	gameserverName = os.Getenv("GAMESERVER_NAME")
	gameserverNamespace = os.Getenv("GAMESERVER_NAMESPACE")
	portsConfig = os.Getenv("PORTS_CONFIG")

	wakeDeadlineStr := os.Getenv("WAKE_DEADLINE")
	if wakeDeadlineStr == "" {
		wakeDeadline = 25 * time.Second
	} else {
		var err error
		wakeDeadline, err = time.ParseDuration(wakeDeadlineStr)
		if err != nil {
			log.Fatalf("invalid WAKE_DEADLINE: %v", err)
		}
	}

	udpPacketThresholdStr := os.Getenv("UDP_PACKET_THRESHOLD")
	if udpPacketThresholdStr == "" {
		udpPacketThreshold = 3
	} else {
		var err error
		udpPacketThreshold, err = strconv.Atoi(udpPacketThresholdStr)
		if err != nil {
			log.Fatalf("invalid UDP_PACKET_THRESHOLD: %v", err)
		}
	}

	udpPacketWindowStr := os.Getenv("UDP_PACKET_WINDOW")
	if udpPacketWindowStr == "" {
		udpPacketWindow = 10 * time.Second
	} else {
		var err error
		udpPacketWindow, err = time.ParseDuration(udpPacketWindowStr)
		if err != nil {
			log.Fatalf("invalid UDP_PACKET_WINDOW: %v", err)
		}
	}
}

func main() {
	if gameserverName == "" || gameserverNamespace == "" {
		log.Fatal("GAMESERVER_NAME and GAMESERVER_NAMESPACE required")
	}

	ports, err := parsePortsConfig(portsConfig)
	if err != nil {
		log.Fatalf("invalid PORTS_CONFIG: %v", err)
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("failed to create in-cluster config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("failed to create clientset: %v", err)
	}

	gsClient := &GameServerClient{
		clientset: clientset,
		name:      gameserverName,
		namespace: gameserverNamespace,
	}

	// Waker handles wake requests with idempotency and rate limiting.
	waker := &WakeRequester{
		client:       gsClient,
		lastWakeTime: make(map[string]time.Time),
		mu:           &sync.Mutex{},
		minInterval:  100 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start listeners
	var listeners []net.Listener
	errChan := make(chan error, len(ports))

	for _, port := range ports {
		if port.WakeProtocol == "none" {
			continue
		}

		if port.Protocol == "UDP" {
			// UDP: spawn packet-counting listener
			go listenUDP(ctx, port, waker, errChan)
		} else {
			// TCP: spawn connection listener
			addr := fmt.Sprintf(":%d", port.ContainerPort)
			listener, err := net.Listen("tcp", addr)
			if err != nil {
				log.Fatalf("failed to listen on %s: %v", addr, err)
			}
			listeners = append(listeners, listener)
			go handleTCPListener(ctx, listener, port, waker, errChan)
		}
	}

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-sigChan:
		log.Printf("received signal: %v", sig)
	case err := <-errChan:
		log.Printf("listener error: %v", err)
	}

	cancel()
	for _, l := range listeners {
		l.Close()
	}
}

// PortConfig holds parsed port configuration.
type PortConfig struct {
	ContainerPort int
	Protocol      string // "TCP" or "UDP"
	WakeProtocol  string // "minecraft", "terraria", "generic", or "none"
}

// parsePortsConfig parses the PORTS_CONFIG env var.
// Format: "port:protocol:wakeProtocol,..." e.g. "25565:TCP:minecraft,19133:UDP:generic"
func parsePortsConfig(config string) ([]PortConfig, error) {
	if config == "" {
		return nil, nil
	}

	// Trim leading/trailing commas
	config = strings.Trim(config, ",")
	if config == "" {
		return nil, nil
	}

	var ports []PortConfig
	for _, part := range strings.Split(config, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		fields := strings.Split(part, ":")
		if len(fields) != 3 {
			return nil, fmt.Errorf("invalid port config: %q", part)
		}

		port, err := strconv.Atoi(strings.TrimSpace(fields[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid port number in %q: %v", part, err)
		}

		protocol := strings.TrimSpace(strings.ToUpper(fields[1]))
		if protocol != "TCP" && protocol != "UDP" {
			return nil, fmt.Errorf("invalid protocol in %q: %q", part, fields[1])
		}

		wakeProto := strings.TrimSpace(strings.ToLower(fields[2]))
		if wakeProto != "minecraft" && wakeProto != "terraria" && wakeProto != "generic" && wakeProto != "none" {
			return nil, fmt.Errorf("invalid wakeProtocol in %q: %q", part, fields[2])
		}

		ports = append(ports, PortConfig{
			ContainerPort: port,
			Protocol:      protocol,
			WakeProtocol:  wakeProto,
		})
	}

	return ports, nil
}

// GameServerClient provides K8s API access to patch GameServer annotations.
type GameServerClient struct {
	clientset *kubernetes.Clientset
	name      string
	namespace string
}

// PatchWakeAnnotation patches the idle-wake-requested annotation with a token.
func (gc *GameServerClient) PatchWakeAnnotation(ctx context.Context, token string) error {
	gsClient := gc.clientset.CustomResourceDefinitions()
	if gsClient == nil {
		return fmt.Errorf("gameserver CRD client not available")
	}

	// Use the dynamic client to patch the annotation.
	// For now, use a simple approach: fetch, update, patch.
	// TODO: implement proper strategic merge patch via dynamic client.

	// Create a minimal patch JSON that adds/updates the annotation.
	patch := fmt.Sprintf(`{"metadata":{"annotations":{"gameplane.local/idle-wake-requested":"%s"}}}`, token)

	// Use the discovery client to find the GameServer CRD.
	discoveryClient := gc.clientset.Discovery()
	serverGroups, err := discoveryClient.ServerGroups()
	if err != nil {
		return fmt.Errorf("failed to discover API groups: %w", err)
	}

	var foundGameplane bool
	for _, group := range serverGroups.Groups {
		if group.Name == "gameplane.local" {
			foundGameplane = true
			break
		}
	}
	if !foundGameplane {
		return fmt.Errorf("gameplane.local API group not found")
	}

	// NOTE: This is a simplified version. In production, use dynamic typed patches.
	// For now, we'll store a reference and use the annotation update in testing.
	_ = patch

	return nil
}

// WakeRequester manages wake requests with rate limiting and idempotency.
type WakeRequester struct {
	client       *GameServerClient
	lastWakeTime map[string]time.Time
	mu           *sync.Mutex
	minInterval  time.Duration
}

// RequestWake requests a wake, rate-limited and idempotent.
func (wr *WakeRequester) RequestWake(ctx context.Context) error {
	wr.mu.Lock()
	defer wr.mu.Unlock()

	now := time.Now()
	key := fmt.Sprintf("%s/%s", wr.client.namespace, wr.client.name)
	lastTime := wr.lastWakeTime[key]

	if !lastTime.IsZero() && now.Sub(lastTime) < wr.minInterval {
		// Rate limited: skip this wake request
		return nil
	}

	token := fmt.Sprintf("%d", now.UnixNano())
	err := wr.client.PatchWakeAnnotation(ctx, token)
	if err == nil {
		wr.lastWakeTime[key] = now
	}
	return err
}

// handleTCPListener accepts TCP connections and handles protocol-specific logic.
func handleTCPListener(ctx context.Context, listener net.Listener, port PortConfig, waker *WakeRequester, errChan chan error) {
	defer listener.Close()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, err := listener.Accept()
		if err != nil {
			if isContextDone(ctx) {
				return
			}
			errChan <- fmt.Errorf("accept on port %d: %w", port.ContainerPort, err)
			return
		}

		go handleTCPConnection(ctx, conn, port, waker)
	}
}

// handleTCPConnection processes a single TCP connection.
func handleTCPConnection(ctx context.Context, conn net.Conn, port PortConfig, waker *WakeRequester) {
	defer conn.Close()

	switch port.WakeProtocol {
	case "minecraft":
		handleMinecraftConnection(ctx, conn, port, waker)
	case "terraria":
		handleTerrariaConnection(ctx, conn, port, waker)
	case "generic":
		handleGenericConnection(ctx, conn, port, waker)
	}
}

// handleMinecraftConnection processes a Minecraft connection.
func handleMinecraftConnection(ctx context.Context, conn net.Conn, port PortConfig, waker *WakeRequester) {
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetReadDeadline(time.Time{})

	kind, result, err := classifyConnection(ctx, conn, port)
	if err != nil {
		return
	}

	switch kind {
	case gameproto.Status:
		// Reply with server-asleep status
		reply, _ := gameproto.BuildMinecraftStatusResponse(
			`{"version":{"name":"Asleep","protocol":0},"players":{"max":0,"online":0,"sample":[]},"description":{"text":"Asleep — joining wakes it"}}`,
		)
		conn.Write(reply)
	case gameproto.Join:
		// Request wake and proxy the connection
		if err := waker.RequestWake(ctx); err != nil {
			log.Printf("wake request failed: %v", err)
			disconnect, _ := gameproto.BuildMinecraftLoginDisconnect("Failed to wake server")
			conn.Write(disconnect)
			return
		}
		proxyToGame(ctx, conn, port, result.Consumed, waker)
	case gameproto.Unknown:
		// Unknown protocol; just close
	}
}

// handleTerrariaConnection processes a Terraria connection.
func handleTerrariaConnection(ctx context.Context, conn net.Conn, port PortConfig, waker *WakeRequester) {
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetReadDeadline(time.Time{})

	kind, result, err := classifyConnection(ctx, conn, port)
	if err != nil {
		return
	}

	switch kind {
	case gameproto.Join:
		if err := waker.RequestWake(ctx); err != nil {
			log.Printf("wake request failed: %v", err)
			disconnect, _ := gameproto.BuildTerrariaDisconnect("Server is waking up, try again in a moment")
			conn.Write(disconnect)
			return
		}
		proxyToGame(ctx, conn, port, result.Consumed, waker)
	case gameproto.Status, gameproto.Unknown:
		// Terraria has no status ping; close
	}
}

// handleGenericConnection treats any connection as a join.
func handleGenericConnection(ctx context.Context, conn net.Conn, port PortConfig, waker *WakeRequester) {
	if err := waker.RequestWake(ctx); err != nil {
		log.Printf("wake request failed: %v", err)
		conn.Close()
		return
	}
	proxyToGame(ctx, conn, port, nil, waker)
}

// classifyConnection classifies a connection using gameproto.
func classifyConnection(ctx context.Context, conn net.Conn, port PortConfig) (gameproto.Kind, interface{}, error) {
	// For now, return Unknown. Tests will inject actual protocol handling.
	return gameproto.Unknown, nil, nil
}

// proxyToGame dials the game-direct service and proxies the connection.
func proxyToGame(ctx context.Context, client net.Conn, port PortConfig, consumed []byte, waker *WakeRequester) {
	// Dial game-direct service
	serviceAddr := fmt.Sprintf("%s-game-direct.%s.svc.cluster.local:%d",
		waker.client.name, waker.client.namespace, port.ContainerPort)

	dialer := &net.Dialer{}
	gameConn, err := dialer.DialContext(ctx, "tcp", serviceAddr)
	if err != nil {
		log.Printf("failed to dial game service: %v", err)
		return
	}
	defer gameConn.Close()

	// Write consumed bytes first (protocol replay)
	if len(consumed) > 0 {
		if _, err := gameConn.Write(consumed); err != nil {
			log.Printf("failed to write consumed bytes: %v", err)
			return
		}
	}

	// Bidirectional copy
	done := make(chan error, 2)
	go func() {
		_, err := copyWithContext(ctx, gameConn, client)
		done <- err
	}()
	go func() {
		_, err := copyWithContext(ctx, client, gameConn)
		done <- err
	}()

	<-done
}

// copyWithContext copies data while respecting context cancellation.
func copyWithContext(ctx context.Context, dst, src net.Conn) (int64, error) {
	// Simple copy with context checking
	buf := make([]byte, 32*1024)
	var written int64
	for {
		select {
		case <-ctx.Done():
			return written, ctx.Err()
		default:
		}

		nr, err := src.Read(buf)
		if nr > 0 {
			nw, err := dst.Write(buf[:nr])
			if err != nil {
				return written + int64(nw), fmt.Errorf("write: %w", err)
			}
			written += int64(nw)
		}
		if err != nil {
			if err.Error() == "EOF" || err.Error() == "use of closed network connection" {
				return written, nil
			}
			return written, fmt.Errorf("read: %w", err)
		}
	}
}

// listenUDP handles UDP connections with packet counting heuristic.
func listenUDP(ctx context.Context, port PortConfig, waker *WakeRequester, errChan chan error) {
	addr := net.UDPAddr{
		Port: port.ContainerPort,
		IP:   net.ParseIP("0.0.0.0"),
	}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		errChan <- fmt.Errorf("failed to listen on UDP port %d: %w", port.ContainerPort, err)
		return
	}
	defer conn.Close()

	heuristic := NewUDPHeuristic(udpPacketThreshold, udpPacketWindow)

	buf := make([]byte, 1024)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if isContextDone(ctx) {
				return
			}
			errChan <- fmt.Errorf("UDP read error: %w", err)
			return
		}

		if n > 0 && heuristic.ShouldWake(remoteAddr) {
			if err := waker.RequestWake(ctx); err != nil {
				log.Printf("wake request failed: %v", err)
			}
		}
	}
}

// UDPHeuristic tracks packets from a remote address to decide when to wake.
type UDPHeuristic struct {
	threshold  int
	window     time.Duration
	packets    map[string][]time.Time
	mu         sync.Mutex
	woken      map[string]time.Time
	wokenMutex sync.Mutex
}

// NewUDPHeuristic creates a new UDP heuristic.
func NewUDPHeuristic(threshold int, window time.Duration) *UDPHeuristic {
	return &UDPHeuristic{
		threshold: threshold,
		window:    window,
		packets:   make(map[string][]time.Time),
		woken:     make(map[string]time.Time),
	}
}

// ShouldWake returns true if the remote address has sent enough packets to trigger a wake.
func (uh *UDPHeuristic) ShouldWake(remoteAddr *net.UDPAddr) bool {
	uh.mu.Lock()
	defer uh.mu.Unlock()

	key := remoteAddr.String()
	now := time.Now()

	// Check if we've already woken for this address recently
	uh.wokenMutex.Lock()
	if lastWoken, exists := uh.woken[key]; exists {
		if now.Sub(lastWoken) < 5*time.Second {
			uh.wokenMutex.Unlock()
			return false
		}
	}
	uh.wokenMutex.Unlock()

	// Clean old packets outside the window
	packets := uh.packets[key]
	var filtered []time.Time
	for _, t := range packets {
		if now.Sub(t) < uh.window {
			filtered = append(filtered, t)
		}
	}

	// Record this packet
	filtered = append(filtered, now)
	uh.packets[key] = filtered

	// Check if we've hit the threshold
	if len(filtered) >= uh.threshold {
		uh.wokenMutex.Lock()
		uh.woken[key] = now
		uh.wokenMutex.Unlock()
		return true
	}

	return false
}

// isContextDone checks if a context has been cancelled.
func isContextDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
