package logger

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"go-job/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var globalLogger *zap.Logger

func New(cfg config.LogConfig) (*zap.Logger, error) {
	dir := filepath.Dir(cfg.Filename)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create log dir failed: %w", err)
		}
	}

	level := zapcore.InfoLevel
	if err := level.Set(cfg.Level); err != nil {
		return nil, fmt.Errorf("invalid zap level: %w", err)
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "time"
	encoderCfg.MessageKey = "msg"
	encoderCfg.LevelKey = "level"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeLevel = zapcore.LowercaseLevelEncoder

	writeSyncer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   cfg.Filename,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	})

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.NewMultiWriteSyncer(writeSyncer, zapcore.AddSync(os.Stdout)),
		level,
	)

	return zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1)), nil
}

func Init(cfg config.LogConfig) error {
	l, err := New(cfg)
	if err != nil {
		return err
	}
	globalLogger = l
	return nil
}

func L() *zap.Logger {
	if globalLogger == nil {
		return zap.NewNop()
	}
	return globalLogger
}

func Sync() error {
	if globalLogger == nil {
		return nil
	}
	return globalLogger.Sync()
}

func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		fields := []zap.Field{
			zap.Int("status", statusCode),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("query", c.Request.URL.RawQuery),
			zap.String("client_ip", c.ClientIP()),
			zap.Duration("latency", latency),
		}

		if len(c.Errors) > 0 {
			L().Error("http request", append(fields, zap.String("errors", c.Errors.String()))...)
			return
		}

		if statusCode >= http.StatusInternalServerError {
			L().Error("http request", fields...)
			return
		}
		L().Info("http request", fields...)
	}
}
