package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "go-job/api/swagger"
	"go-job/internal/app/handler"
	"go-job/internal/app/middleware"
	"go-job/internal/repository"
	"go-job/internal/service"
	conf "go-job/pkg/config"
	"go-job/pkg/database"
	"go-job/pkg/logger"
	"go-job/pkg/response"
	"go-job/pkg/timer"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	configPath := "config/dev.yaml"
	if v := os.Getenv("GO_JOB_CONFIG"); v != "" {
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
	r.Use(middleware.CORS(middleware.DefaultCORSConfig()))
	r.Use(middleware.RequestLogger())
	r.Use(middleware.Recovery())

	r.GET("/health", func(c *gin.Context) {
		response.Success(c, gin.H{
			"status": "ok",
			"mysql":  "connected",
			"redis":  "connected",
		})
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Dependency wiring: repository -> service -> handler
	db := database.MySQL()
	redisClient := database.Redis()

	jobRepo := repository.NewJobRepository(db)
	executorRepo := repository.NewExecutorRepository(db)
	logRepo := repository.NewLogRepository(db)

	logService := service.NewLogService(logRepo)
	triggerService := service.NewTriggerService(logService)

	timeWheel, err := timer.NewTimeWheel(1*time.Second, 60)
	if err != nil {
		log.Fatal("create time wheel failed", zap.Error(err))
	}
	triggerCallback := service.NewTriggerCallbackFromService(triggerService, "scheduler-local", 0, nil)
	scheduleService := service.NewScheduleServiceWithLock(jobRepo, timeWheel, triggerCallback, redisClient)

	jobService := service.NewJobService(jobRepo, scheduleService)
	executorService := service.NewExecutorService(executorRepo)

	jobHandler := handler.NewJobHandler(jobService)
	executorHandler := handler.NewExecutorHandler(executorService)
	logHandler := handler.NewLogHandler(logService)
	triggerHandler := handler.NewTriggerHandler(jobService, triggerService)

	api := r.Group("/api/v1")
	jobHandler.RegisterRoutes(api)
	executorHandler.RegisterRoutes(api)
	logHandler.RegisterRoutes(api)
	triggerHandler.RegisterRoutes(api)

	// Start scheduler after dependencies and routes are ready.
	if err := scheduleService.Start(context.Background()); err != nil {
		log.Fatal("start schedule service failed", zap.Error(err))
	}

	srv := &http.Server{
		Addr:           ":" + cfg.Server.Port,
		Handler:        r,
		ReadTimeout:    time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(cfg.Server.WriteTimeout) * time.Second,
		MaxHeaderBytes: cfg.Server.MaxHeaderBytes,
	}

	go func() {
		log.Info("server starting", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server start failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("server shutdown failed", zap.Error(err))
		return
	}
	if err := scheduleService.Stop(); err != nil {
		log.Error("schedule service stop failed", zap.Error(err))
	}
	log.Info("server stopped")
}
