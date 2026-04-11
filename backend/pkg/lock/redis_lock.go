package lock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	// ErrLockNotAcquired 加锁失败（其他持有者已占用）。
	ErrLockNotAcquired = errors.New("lock: not acquired")
	// ErrLockNotHeld 解锁失败：键不存在、已过期或 token 不匹配（防止误删他人锁）。
	ErrLockNotHeld = errors.New("lock: not held or token mismatch")
	// ErrInvalidConfig 构造参数非法。
	ErrInvalidConfig = errors.New("lock: invalid config")
)

// unlockScript 原子比较 token 后删除，避免释放非本实例持有的锁。
const unlockScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
else
	return 0
end
`

// RedisLock 基于 Redis SET NX + TTL 的分布式锁，释放使用 Lua 保证原子性。
// 每个实例在创建时生成唯一 token，解锁时仅当 value 与 token 一致才 DEL。
type RedisLock struct {
	client *redis.Client
	key    string
	token  string
	ttl    time.Duration
}

// NewRedisLock 创建锁句柄（尚未占用 Redis 键）。key 建议带业务前缀，如 "go_job:trigger:123"。
func NewRedisLock(client *redis.Client, key string, ttl time.Duration) (*RedisLock, error) {
	if client == nil {
		return nil, ErrInvalidConfig
	}
	if key == "" {
		return nil, ErrInvalidConfig
	}
	if ttl <= 0 {
		return nil, ErrInvalidConfig
	}
	token, err := randomToken(16)
	if err != nil {
		return nil, err
	}
	return &RedisLock{
		client: client,
		key:    key,
		token:  token,
		ttl:    ttl,
	}, nil
}

func randomToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Key 返回锁在 Redis 中的键名。
func (l *RedisLock) Key() string { return l.key }

// Token 返回本实例的锁令牌（一般仅用于调试，不要写入日志全量明文到不可信环境）。
func (l *RedisLock) Token() string { return l.token }

// Lock 尝试加锁：SET key token NX EX ttl（由 go-redis SetNX 映射为原子命令）。
// 返回 true 表示加锁成功，false 表示键已被占用。
func (l *RedisLock) Lock(ctx context.Context) (bool, error) {
	ok, err := l.client.SetNX(ctx, l.key, l.token, l.ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

// MustLock 与 Lock 相同，但失败时返回 ErrLockNotAcquired。
func (l *RedisLock) MustLock(ctx context.Context) error {
	ok, err := l.Lock(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return ErrLockNotAcquired
	}
	return nil
}

// Unlock 使用 Lua 脚本：仅当 GET 值等于本实例 token 时 DEL。
// 成功返回 nil；键不存在或 token 不匹配返回 ErrLockNotHeld。
func (l *RedisLock) Unlock(ctx context.Context) error {
	res, err := l.client.Eval(ctx, unlockScript, []string{l.key}, l.token).Int64()
	if err != nil {
		return err
	}
	if res != 1 {
		return ErrLockNotHeld
	}
	return nil
}

// UnlockBestEffort 解锁；若未持有或已过期则忽略错误，适合 defer 场景。
func (l *RedisLock) UnlockBestEffort(ctx context.Context) {
	_ = l.Unlock(ctx)
}

// Refresh 在仍持有锁时延长 TTL（GET 比对 token 后 PEXPIRE，原子脚本）。
const refreshScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("PEXPIRE", KEYS[1], ARGV[2])
else
	return 0
end
`

// Refresh 续期；仅当当前值仍为本 token 时延长。ttl 必须 > 0。
func (l *RedisLock) Refresh(ctx context.Context, ttl time.Duration) error {
	if ttl <= 0 {
		return ErrInvalidConfig
	}
	ms := ttl.Milliseconds()
	if ms < 1 {
		ms = 1
	}
	res, err := l.client.Eval(ctx, refreshScript, []string{l.key}, l.token, ms).Int64()
	if err != nil {
		return err
	}
	if res != 1 {
		return ErrLockNotHeld
	}
	return nil
}
