package security

import (
	"sync"
	"time"
)

type rlEntry struct {
	failures   []time.Time
	lockedTill time.Time
}

// RateLimiter 内存级 sliding-window 限速器，支持 lockout 机制。
// 适用场景：登录失败计数（每 IP / 每用户名）。
type RateLimiter struct {
	mu      sync.Mutex
	max     int
	window  time.Duration
	lockout time.Duration
	entries map[string]*rlEntry
	clock   func() time.Time
}

// NewRateLimiter 创建限速器。max 次失败 / window 时间窗内触发 lockout。
func NewRateLimiter(max int, window, lockout time.Duration) *RateLimiter {
	return &RateLimiter{
		max:     max,
		window:  window,
		lockout: lockout,
		entries: make(map[string]*rlEntry),
		clock:   time.Now,
	}
}

// Allow 返回 key 当前是否允许尝试。不消耗配额，只查看。
// 如果 key 处于 lockout 期间，返回 false。
// 如果窗口内失败次数已达 max，也返回 false。
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	e, ok := rl.entries[key]
	if !ok {
		return true
	}
	now := rl.clock()
	if now.Before(e.lockedTill) {
		return false
	}
	rl.pruneLocked(e, now)
	return len(e.failures) < rl.max
}

// RecordFailure 记一次失败。达到 max 后进入 lockout。
func (rl *RateLimiter) RecordFailure(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := rl.clock()
	e := rl.entries[key]
	if e == nil {
		e = &rlEntry{}
		rl.entries[key] = e
	}
	rl.pruneLocked(e, now)
	e.failures = append(e.failures, now)
	if len(e.failures) >= rl.max {
		e.lockedTill = now.Add(rl.lockout)
	}
}

// RecordSuccess 清空 key 的计数（例如登录成功后调用）。
func (rl *RateLimiter) RecordSuccess(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.entries, key)
}

func (rl *RateLimiter) pruneLocked(e *rlEntry, now time.Time) {
	cut := now.Add(-rl.window)
	keep := e.failures[:0]
	for _, t := range e.failures {
		if t.After(cut) {
			keep = append(keep, t)
		}
	}
	e.failures = keep
}
