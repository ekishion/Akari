//go:build windows

package config

import (
	"log"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// InitSystemProxy detects and applies Windows system proxy settings
// if HTTP_PROXY or HTTPS_PROXY environment variables are not already set.
func InitSystemProxy() {
	if os.Getenv("HTTP_PROXY") != "" || os.Getenv("HTTPS_PROXY") != "" || os.Getenv("http_proxy") != "" || os.Getenv("https_proxy") != "" {
		return
	}

	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return
	}
	defer k.Close()

	enable, _, err := k.GetIntegerValue("ProxyEnable")
	if err == nil && enable == 1 {
		server, _, err := k.GetStringValue("ProxyServer")
		if err == nil && server != "" {
			proxyUrl := server
			if strings.Contains(server, ";") {
				parts := strings.Split(server, ";")
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if strings.HasPrefix(strings.ToLower(p), "http=") {
						proxyUrl = strings.TrimPrefix(p, "http=")
						break
					}
				}
			}
			if !strings.HasPrefix(proxyUrl, "http://") && !strings.HasPrefix(proxyUrl, "https://") {
				proxyUrl = "http://" + proxyUrl
			}
			_ = os.Setenv("HTTP_PROXY", proxyUrl)
			_ = os.Setenv("HTTPS_PROXY", proxyUrl)
			log.Printf("[Proxy] Detected Windows system proxy: %s (Auto-applied)", proxyUrl)
		}
	}
}
