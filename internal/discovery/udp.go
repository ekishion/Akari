package discovery

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"

	"akari-bridge/internal/config"
)

type ServerDiscoveryResponse struct {
	Address         string `json:"Address"`
	Id              string `json:"Id"`
	Name            string `json:"Name"`
	EndpointAddress string `json:"EndpointAddress,omitempty"`
}

type UdpServer struct {
	cfg  *config.Config
	conn *net.UDPConn
	stop chan struct{}
}

func NewUdpServer(cfg *config.Config) *UdpServer {
	return &UdpServer{
		cfg:  cfg,
		stop: make(chan struct{}),
	}
}

func (s *UdpServer) Start() error {
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("0.0.0.0:%d", s.cfg.UdpPort))
	if err != nil {
		return fmt.Errorf("failed to resolve UDP addr: %w", err)
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return fmt.Errorf("failed to listen UDP on port %d: %w", s.cfg.UdpPort, err)
	}
	s.conn = conn

	log.Printf("[Discovery] Emby UDP discovery service listening on port %d", s.cfg.UdpPort)

	go s.listenLoop()
	return nil
}

func (s *UdpServer) Stop() {
	close(s.stop)
	if s.conn != nil {
		_ = s.conn.Close()
	}
}

func (s *UdpServer) listenLoop() {
	buf := make([]byte, 2048)
	for {
		select {
		case <-s.stop:
			return
		default:
		}

		n, remoteAddr, err := s.conn.ReadFrom(buf)
		if err != nil {
			select {
			case <-s.stop:
				return
			default:
				// ignore read error on close
				continue
			}
		}

		msg := strings.TrimSpace(string(buf[:n]))
		// Emby clients send "who is EmbyServer?" or similar search probes
		if strings.Contains(strings.ToLower(msg), "who is embyserver?") || strings.Contains(strings.ToLower(msg), "embyserver") {
			s.respondToProbe(remoteAddr)
		}
	}
}

func (s *UdpServer) respondToProbe(remoteAddr net.Addr) {
	serverUrl := s.cfg.GetServerUrl()
	if udpAddr, ok := remoteAddr.(*net.UDPAddr); ok && udpAddr.IP != nil {
		if conn, err := net.Dial("udp", udpAddr.String()); err == nil {
			if local, ok := conn.LocalAddr().(*net.UDPAddr); ok && local.IP != nil {
				serverUrl = fmt.Sprintf("http://%s:%d", local.IP.String(), s.cfg.HttpPort)
			}
			_ = conn.Close()
		}
	}

	resp := ServerDiscoveryResponse{
		Address:         serverUrl,
		Id:              s.cfg.ServerId,
		Name:            s.cfg.ServerName,
		EndpointAddress: serverUrl,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return
	}

	_, _ = s.conn.WriteTo(data, remoteAddr)
	log.Printf("[Discovery] Responded to probe from %s with server %s (%s)", remoteAddr.String(), s.cfg.ServerName, serverUrl)
}
