package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HttpPort      int
	UdpPort       int
	ServerName    string
	ServerId      string
	DataDir       string
	AdminUsername string
	AdminPassword string
	PublicHost       string
	BangumiHost      string
	BangumiImageHost string
	DanDanHost       string
	EnableECH        bool
}

func LoadConfig() (*Config, error) {
	InitSystemProxy()

	dataDir := getEnv("DATA_DIR", "./data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	serverId := getOrCreateServerId(dataDir)

	httpPort := getEnvInt("PORT", 8096)
	if httpPort == 8096 {
		httpPort = getEnvInt("HTTP_PORT", 8096)
	}

	cfg := &Config{
		HttpPort:         httpPort,
		UdpPort:          getEnvInt("UDP_PORT", 7359),
		ServerName:       getEnv("SERVER_NAME", "Akari Media"),
		ServerId:         serverId,
		DataDir:          dataDir,
		AdminUsername:    getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword:    getEnv("ADMIN_PASSWORD", ""),
		PublicHost:       getEnv("PUBLIC_HOST", ""),
		BangumiHost:      getEnv("BANGUMI_HOST", "https://api.bgm.tv"),
		BangumiImageHost: getEnv("BANGUMI_IMAGE_HOST", "https://lain.bgm.tv"),
		DanDanHost:       getEnv("DANDAN_HOST", "https://api.dandanplay.net"),
		EnableECH:        getEnv("ENABLE_ECH", "true") == "true" || getEnv("ENABLE_ECH", "1") == "1",
	}

	return cfg, nil
}

func GetSystemProxy() string {
	if p := os.Getenv("HTTP_PROXY"); p != "" {
		return p
	}
	if p := os.Getenv("HTTPS_PROXY"); p != "" {
		return p
	}
	return ""
}

func (c *Config) GetServerUrl() string {
	if c.PublicHost != "" {
		if strings.HasPrefix(c.PublicHost, "http://") || strings.HasPrefix(c.PublicHost, "https://") {
			return c.PublicHost
		}
		return fmt.Sprintf("http://%s:%d", c.PublicHost, c.HttpPort)
	}

	localIP := getLocalIP()
	return fmt.Sprintf("http://%s:%d", localIP, c.HttpPort)
}

// GetBaseURLFromRequest dynamically determines the base URL (scheme + host:port)
// from the client's HTTP request. If r is nil or r.Host is empty, falls back to GetServerUrl().
func (c *Config) GetBaseURLFromRequest(r *http.Request) string {
	if r != nil && r.Host != "" {
		scheme := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		return fmt.Sprintf("%s://%s", scheme, r.Host)
	}
	return c.GetServerUrl()
}

func getOrCreateServerId(dataDir string) string {
	idFile := filepath.Join(dataDir, "server_id.txt")
	if data, err := os.ReadFile(idFile); err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id
		}
	}

	// Generate 32-char hex string as Emby Server ID
	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes)
	newId := hex.EncodeToString(bytes)

	_ = os.WriteFile(idFile, []byte(newId), 0644)
	return newId
}

func getLocalIP() string {
	// 1. Try to query OS routing table via dummy UDP dial
	// This does NOT send any packet over the network, but determines the local interface for the default route.
	conn, err := net.DialTimeout("udp", "223.5.5.5:80", 500*time.Millisecond)
	if err == nil {
		defer conn.Close()
		if localAddr, ok := conn.LocalAddr().(*net.UDPAddr); ok && !localAddr.IP.IsLoopback() {
			if ip4 := localAddr.IP.To4(); ip4 != nil {
				return ip4.String()
			}
		}
	}

	// 2. Fallback: inspect network interfaces
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		var lanCandidates []string
		var otherCandidates []string

		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
				if ip4 := ipNet.IP.To4(); ip4 != nil {
					// Exclude link-local, hotspot, virtualbox, and docker/wsl subnets
					if ip4[0] == 169 && ip4[1] == 254 {
						continue
					}
					if ip4[0] == 192 && ip4[1] == 168 && (ip4[2] == 137 || ip4[2] == 56) {
						continue
					}
					if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
						continue
					}

					ipStr := ip4.String()
					if ip4[0] == 192 || ip4[0] == 10 {
						lanCandidates = append(lanCandidates, ipStr)
					} else {
						otherCandidates = append(otherCandidates, ipStr)
					}
				}
			}
		}

		if len(lanCandidates) > 0 {
			return lanCandidates[0]
		}
		if len(otherCandidates) > 0 {
			return otherCandidates[0]
		}
	}
	return "127.0.0.1"
}

func getEnv(key, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}

func getEnvInt(key string, def int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return def
}
