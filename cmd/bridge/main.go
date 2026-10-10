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
	"akari-bridge/internal/bilibili"
	"akari-bridge/internal/config"
	adminctrl "akari-bridge/internal/controller/admin"
	embyctrl "akari-bridge/internal/controller/emby"
	streamctrl "akari-bridge/internal/controller/stream"
	"akari-bridge/internal/danmaku"
	"akari-bridge/internal/discovery"
	"akari-bridge/internal/engine"
	"akari-bridge/internal/proxy"
	"akari-bridge/internal/repository"
	"akari-bridge/internal/resolver"
	"akari-bridge/internal/router"
	"akari-bridge/internal/rules"
	"akari-bridge/internal/service"
	"akari-bridge/internal/storage"
)

func main() {
	log.Println("==================================================")
	log.Println("           Starting Akari Media (Emby Bridge)     ")
	log.Println("==================================================")

	// 1. Configuration Bootstrap
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

	// 3. Database & Granular Repositories (Data Persistence Layer)
	legacyDB, err := storage.OpenDB(cfg)
	if err != nil {
		log.Fatalf("[Error] Failed to initialize SQLite database: %v", err)
	}
	defer legacyDB.Close()

	repoConn, err := repository.OpenDB(cfg)
	if err != nil {
		log.Fatalf("[Error] Failed to initialize SQLite repository pool: %v", err)
	}
	defer repoConn.Close()

	userRepo := repository.NewUserRepository(repoConn)
	playbackRepo := repository.NewPlaybackRepository(repoConn)
	favRepo := repository.NewFavoriteRepository(repoConn)
	settingRepo := repository.NewSettingRepository(repoConn)
	ruleRepo := repository.NewRuleRepository(repoConn)
	synonymRepo := repository.NewSynonymRepository(repoConn)
	activityRepo := repository.NewActivityRepository(repoConn)

	// 4. Infrastructure & Domain Clients
	bgmClient := bangumi.NewClient(cfg.BangumiHost)
	danmakuClient := danmaku.NewClient(cfg.DanDanHost)
	ruleMgr := rules.NewRuleManager(cfg)
	eng := engine.NewEngine()
	res := resolver.NewStreamResolver()
	streamProxy := proxy.NewStreamProxy(cfg, legacyDB)

	authSvc := auth.NewAuthService(cfg, legacyDB)
	adminAuth := auth.NewAdminAuthService(cfg, legacyDB)

	// 5. Bilibili Subsystem
	biliClient := bilibili.NewClient(nil)
	var initialCreds *bilibili.BilibiliCredentials
	var initialSettings *bilibili.BilibiliSettings
	if legacyDB != nil {
		if c, err := legacyDB.GetBilibiliCredentials(); err == nil && c != nil {
			initialCreds = &bilibili.BilibiliCredentials{
				SessData:   c.SessData,
				BiliJct:    c.BiliJct,
				Buvid3:     c.Buvid3,
				DedeUserID: c.DedeUserID,
			}
		}
		if s, err := legacyDB.GetBilibiliSettings(); err == nil && s != nil {
			initialSettings = &bilibili.BilibiliSettings{
				Enabled:        s.Enabled,
				PreferBilibili: s.PreferBilibili,
				MaxQuality:     s.MaxQuality,
				StreamMode:     s.StreamMode,
			}
		}
	}
	saveCredsFunc := func(creds *bilibili.BilibiliCredentials) error {
		if legacyDB == nil {
			return nil
		}
		if creds == nil {
			return legacyDB.SaveBilibiliCredentials("", "", "", "")
		}
		return legacyDB.SaveBilibiliCredentials(creds.SessData, creds.BiliJct, creds.Buvid3, creds.DedeUserID)
	}
	saveSettingsFunc := func(st *bilibili.BilibiliSettings) error {
		if legacyDB == nil || st == nil {
			return nil
		}
		return legacyDB.SaveBilibiliSettings(st.Enabled, st.PreferBilibili, st.MaxQuality, st.StreamMode)
	}

	biliAuth := bilibili.NewAuthManager(biliClient, initialCreds, initialSettings, saveCredsFunc, saveSettingsFunc)
	biliResolver := bilibili.NewResolver(biliClient)
	dashMuxer := bilibili.NewDASHMuxer()

	// 6. Business Orchestration (Service Layer)
	playbackSvc := service.NewPlaybackService(cfg, bgmClient, ruleMgr, eng, res, playbackRepo, synonymRepo, biliAuth, biliResolver, dashMuxer)
	catalogSvc := service.NewCatalogService(cfg, bgmClient, playbackRepo, favRepo, userRepo)
	adminSvc := service.NewAdminService(cfg, ruleMgr, eng, activityRepo, userRepo)

	log.Printf("[Init] Loaded %d enabled rule(s)", len(ruleMgr.GetEnabledPlugins()))
	log.Printf("[Init] DanDanPlay endpoint: %s", cfg.DanDanHost)
	log.Printf("[Init] Clean Architecture repositories initialized in %s", cfg.DataDir)
	_ = ruleRepo // Referenced for compilation

	// 7. Presentation Layer (Controllers)
	embyItemCtrl := embyctrl.NewItemController(cfg, catalogSvc, bgmClient, authSvc, playbackRepo)
	embyPbCtrl := embyctrl.NewPlaybackController(cfg, playbackSvc, danmakuClient, bgmClient)
	embyUserCtrl := embyctrl.NewUserController(authSvc, playbackRepo, favRepo, userRepo, activityRepo)
	embySysCtrl := embyctrl.NewSystemController(cfg)

	adminAuthCtrl := adminctrl.NewAuthController(adminAuth, authSvc, activityRepo)
	adminUserCtrl := adminctrl.NewUserController(authSvc, bgmClient, userRepo)
	adminRuleCtrl := adminctrl.NewRuleController(ruleMgr, eng, adminSvc)
	adminSysCtrl := adminctrl.NewSystemController(cfg, adminSvc, authSvc, ruleMgr, bgmClient, danmakuClient, settingRepo, playbackRepo, userRepo, synonymRepo, activityRepo)
	adminBiliCtrl := adminctrl.NewBilibiliController(biliAuth)

	streamProxyCtrl := streamctrl.NewProxyController(streamProxy, dashMuxer, biliAuth)

	// 8. Routers Setup
	embyRouter := router.NewEmbyRouter(authSvc, embyItemCtrl, embyPbCtrl, embyUserCtrl, embySysCtrl)
	adminRouter := router.NewAdminRouter(adminAuth, adminAuthCtrl, adminUserCtrl, adminRuleCtrl, adminSysCtrl, adminBiliCtrl)
	streamRouter := router.NewStreamRouter(streamProxyCtrl)
	rootRouter := router.NewRouter(cfg, embyRouter, adminRouter, streamRouter, embySysCtrl)

	httpEngine := rootRouter.InitEngine()

	// 9. HTTP Server Lifecyle
	serverAddr := fmt.Sprintf("0.0.0.0:%d", cfg.HttpPort)
	srv := &http.Server{
		Addr:    serverAddr,
		Handler: httpEngine,
	}

	go func() {
		log.Printf("[Server] Emby Bridge HTTP Server listening on %s (URL: %s)", serverAddr, cfg.GetServerUrl())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Error] HTTP server failed: %v", err)
		}
	}()

	// 10. Graceful Shutdown
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
