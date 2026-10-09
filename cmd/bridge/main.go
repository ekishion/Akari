package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"akari-bridge/internal/auth"
	"akari-bridge/internal/bangumi"
	"akari-bridge/internal/config"
	"akari-bridge/internal/danmaku"
	"akari-bridge/internal/discovery"
	"akari-bridge/internal/embyapi"
	"akari-bridge/internal/engine"
	"akari-bridge/internal/proxy"
	"akari-bridge/internal/resolver"
	"akari-bridge/internal/rules"
	"akari-bridge/internal/storage"
)

func main() {
	log.Println("==================================================")
	log.Println("           Starting Akari Media (Emby Bridge)     ")
	log.Println("==================================================")

	// 1. Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("[Error] Failed to load config: %v", err)
	}

	log.Printf("[Config] Server Name: %s", cfg.ServerName)
	log.Printf("[Config] Server ID:   %s", cfg.ServerId)
	log.Printf("[Config] HTTP Port:   %d", cfg.HttpPort)
	log.Printf("[Config] UDP Port:    %d", cfg.UdpPort)
	log.Printf("[Config] Data Dir:    %s", cfg.DataDir)

	// 2. Start UDP 7359 Discovery Service
	udpServer := discovery.NewUdpServer(cfg)
	if err := udpServer.Start(); err != nil {
		log.Printf("[Warn] UDP discovery service could not start: %v", err)
	}
	defer udpServer.Stop()

	// 3. Initialize Storage & Database
	db, err := storage.OpenDB(cfg)
	if err != nil {
		log.Fatalf("[Error] Failed to initialize SQLite database: %v", err)
	}
	defer db.Close()

	// 4. Initialize Services
	authSvc := auth.NewAuthService(cfg, db)
	adminAuth := auth.NewAdminAuthService(cfg, db)
	bgmClient := bangumi.NewClient(cfg.BangumiHost)
	danmakuClient := danmaku.NewClient(cfg.DanDanHost)
	ruleMgr := rules.NewRuleManager(cfg)
	eng := engine.NewEngine()
	res := resolver.NewStreamResolver()
	streamProxy := proxy.NewStreamProxy(cfg, db)

	log.Printf("[Init] Loaded %d enabled rule(s)", len(ruleMgr.GetEnabledPlugins()))
	log.Printf("[Init] DanDanPlay endpoint: %s", cfg.DanDanHost)
	log.Printf("[Init] SQLite storage initialized in %s", cfg.DataDir)

	// 4. Setup Router
	router := embyapi.SetupRouter(cfg, authSvc, adminAuth, bgmClient, ruleMgr, eng, res, streamProxy, danmakuClient, db)

	// 5. Start HTTP Server
	serverAddr := fmt.Sprintf("0.0.0.0:%d", cfg.HttpPort)
	srv := &http.Server{
		Addr:    serverAddr,
		Handler: router,
	}

	go func() {
		log.Printf("[Server] Emby Bridge HTTP Server listening on %s (URL: %s)", serverAddr, cfg.GetServerUrl())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Error] HTTP server failed: %v", err)
		}
	}()

	// 6. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[Server] Shutting down Akari Media gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("[Error] Server forced to shutdown: %v", err)
	}

	log.Println("[Server] Akari Media stopped.")
}
