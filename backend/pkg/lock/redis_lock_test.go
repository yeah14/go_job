package lock

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(func() { mr.Close() })

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	return mr, client
}

// TestRedisLock_ConcurrentAcquireOnlyOne 多 goroutine 同时抢同一把锁，同一时刻只能有一个 SetNX 成功。
func TestRedisLock_ConcurrentAcquireOnlyOne(t *testing.T) {
	_, client := newTestRedis(t)
	ctx := context.Background()
	const (
		goroutines = 64
		rounds     = 20
	)
	lockKey := "go_job:test:mutex"

	for r := 0; r < rounds; r++ {
		var successCount int32
		var wg sync.WaitGroup
		wg.Add(goroutines)
		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				l, err := NewRedisLock(client, lockKey, 5*time.Second)
				if err != nil {
					t.Error(err)
					return
				}
				ok, err := l.Lock(ctx)
				if err != nil {
					t.Error(err)
					return
				}
				if ok {
					atomic.AddInt32(&successCount, 1)
				}
			}()
		}
		wg.Wait()
		if successCount != 1 {
			t.Fatalf("round %d: expected exactly 1 lock winner, got %d", r, successCount)
		}
		// 清理，下一轮重新抢
		_ = client.Del(ctx, lockKey).Err()
	}
}

// TestRedisLock_SerializeAfterUnlock 先解锁再抢，不应死锁，且仍可互斥。
func TestRedisLock_SerializeAfterUnlock(t *testing.T) {
	_, client := newTestRedis(t)
	ctx := context.Background()
	key := "go_job:test:serialize"

	l1, err := NewRedisLock(client, key, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := l1.Lock(ctx); !ok {
		t.Fatal("first lock should succeed")
	}
	if err := l1.Unlock(ctx); err != nil {
		t.Fatalf("unlock: %v", err)
	}

	l2, err := NewRedisLock(client, key, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := l2.Lock(ctx); !ok {
		t.Fatal("after unlock, new instance should acquire")
	}
	if err := l2.Unlock(ctx); err != nil {
		t.Fatalf("unlock2: %v", err)
	}
}

// TestRedisLock_MistakenReleaseDifferentToken 未持有者用 Lua 解锁不得删掉别人的锁（误释放防护）。
func TestRedisLock_MistakenReleaseDifferentToken(t *testing.T) {
	_, client := newTestRedis(t)
	ctx := context.Background()
	key := "go_job:test:mistaken"

	owner, err := NewRedisLock(client, key, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := owner.Lock(ctx); !ok {
		t.Fatal("owner should lock")
	}

	other, err := NewRedisLock(client, key, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := other.Lock(ctx); ok {
		t.Fatal("other should not acquire while owner holds")
	}
	if err := other.Unlock(ctx); !errors.Is(err, ErrLockNotHeld) {
		t.Fatalf("other unlock should be ErrLockNotHeld, got %v", err)
	}

	// 锁仍在 owner
	val, err := client.Get(ctx, key).Result()
	if err != nil || val != owner.Token() {
		t.Fatalf("lock value corrupted or missing: err=%v val=%q want token of owner", err, val)
	}
	if err := owner.Unlock(ctx); err != nil {
		t.Fatalf("owner unlock: %v", err)
	}
}

// TestRedisLock_ExpireThenReacquire TTL 到期后键消失，不应“永久死锁”；他人可重新加锁。
func TestRedisLock_ExpireThenReacquire(t *testing.T) {
	mr, client := newTestRedis(t)
	ctx := context.Background()
	key := "go_job:test:expire"

	l1, err := NewRedisLock(client, key, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := l1.Lock(ctx); !ok {
		t.Fatal("first lock")
	}
	mr.FastForward(200 * time.Millisecond)

	if err := l1.Unlock(ctx); !errors.Is(err, ErrLockNotHeld) {
		t.Fatalf("after expire, unlock should ErrLockNotHeld, got %v", err)
	}

	l2, err := NewRedisLock(client, key, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := l2.Lock(ctx); !ok {
		t.Fatal("after expire, new lock should succeed (no deadlock)")
	}
	_ = l2.Unlock(ctx)
}

// TestRedisLock_SameInstanceDoubleLock 同一实例连续 Lock：第二次应失败（防误以为重入锁）。
func TestRedisLock_SameInstanceDoubleLock(t *testing.T) {
	_, client := newTestRedis(t)
	ctx := context.Background()
	l, err := NewRedisLock(client, "go_job:test:double", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := l.Lock(ctx); !ok {
		t.Fatal("first lock")
	}
	if ok, _ := l.Lock(ctx); ok {
		t.Fatal("second Lock on same key should fail (not reentrant)")
	}
	_ = l.Unlock(ctx)
}

// TestRedisLock_ConcurrentTryThenOneUnlock 多协程抢锁后只有一个持有，Unlock 仅对持有者成功。
func TestRedisLock_ConcurrentTryThenOneUnlock(t *testing.T) {
	_, client := newTestRedis(t)
	ctx := context.Background()
	key := "go_job:test:concurrent-unlock"
	const n = 32

	locks := make([]*RedisLock, n)
	for i := range locks {
		var err error
		locks[i], err = NewRedisLock(client, key, 30*time.Second)
		if err != nil {
			t.Fatal(err)
		}
	}

	var winner *RedisLock
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			ok, err := locks[i].Lock(ctx)
			if err != nil {
				t.Error(err)
				return
			}
			if ok {
				mu.Lock()
				if winner != nil {
					t.Error("multiple winners")
				}
				winner = locks[i]
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if winner == nil {
		t.Fatal("expected one winner")
	}
	for i := 0; i < n; i++ {
		if locks[i] == winner {
			continue
		}
		if err := locks[i].Unlock(ctx); !errors.Is(err, ErrLockNotHeld) {
			t.Fatalf("non-owner %d unlock: want ErrLockNotHeld, got %v", i, err)
		}
	}
	if err := winner.Unlock(ctx); err != nil {
		t.Fatalf("winner unlock: %v", err)
	}
}
