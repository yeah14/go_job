package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	conf "go-job/pkg/config"
	"go-job/pkg/database"
	"go-job/pkg/logger"
	"go-job/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	configPath := "config/executor.yaml"
	if v := os.Getenv("GO_JOB_EXECUTOR_CONFIG"); v != "" {
		configPath = v
	}

	cfg, err := conf.Load(configPath)
	if err != nil {
		panic(err)
	}

	if err := logger.Init(cfg.Log); err != nil {
		panic(err)
	}
	defer func() {
		_ = logger.Sync()
	}()
	log := logger.L()

	if err := database.InitMySQL(cfg.Database); err != nil {
		log.Fatal("mysql init failed", zap.Error(err))
	}
	defer func() {
		_ = database.CloseMySQL()
	}()

	if err := database.InitRedis(cfg.Redis); err != nil {
		log.Fatal("redis init failed", zap.Error(err))
	}
	defer func() {
		_ = database.CloseRedis()
	}()

	gin.SetMode(cfg.Server.Mode)
	r := gin.New()
	r.Use(logger.GinLogger())
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		response.Success(c, gin.H{
			"status": "ok",
			"role":   "executor",
			"mysql":  "connected",
			"redis":  "connected",
		})
	})

	r.GET("/api/v1/executor/ping", func(c *gin.Context) {
		response.Success(c, gin.H{
			"message": "executor is ready",
		})
	})

	srv := &http.Server{
		Addr:           ":" + cfg.Server.Port,
		Handler:        r,
		ReadTimeout:    time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(cfg.Server.WriteTimeout) * time.Second,
		MaxHeaderBytes: cfg.Server.MaxHeaderBytes,
	}

	go func() {
		log.Info("executor starting", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("executor start failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("executor shutdown failed", zap.Error(err))
		return
	}

	log.Info("executor stopped")
}
