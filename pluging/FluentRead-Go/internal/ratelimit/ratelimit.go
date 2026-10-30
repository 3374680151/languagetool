// Package ratelimit 提供进程内的按 key 令牌桶限流。
// 约定：限流只读内存、不读库——它的作用是"挡住瞬间洪峰"，不是记账；
// 记账（日配额）由 store.QuotaRepo 落库，重启后仍然有效。
// 因此这里刻意不做持久化：重启清零是限流的正常语义，不是缺陷。
package ratelimit

import (
	"math"
	"sync"
	"time"
)

// bucket 是单个 key 的令牌桶状态。
type bucket struct {
	tokens float64
	last   time.Time
}

// Limiter 是按 key 的令牌桶限流器。
// 容量等于每分钟允许次数（即允许一次性用满一分钟的额度，这是最宽松也最直观的解释：
// "每分钟 N 次"意味着任意 1 秒内最多突发 N 次）。
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	// rate 是每秒补充的令牌数。
	rate  float64
	burst float64
	now   func() time.Time
	// sweepAt 是下一次清理闲置桶的时间，避免 map 无限增长。
	sweepAt time.Time
}

// New 构造限流器。perMinute<=0 表示不限流，返回的 Limiter 所有 Allow 都放行。
func New(perMinute int) *Limiter {
	if perMinute <= 0 {
		return &Limiter{rate: 0, burst: 0, now: time.Now}
	}
	return &Limiter{
		buckets: make(map[string]*bucket),
		rate:    float64(perMinute) / 60.0,
		burst:   float64(perMinute),
		now:     time.Now,
		sweepAt: time.Now().Add(time.Minute),
	}
}

// Allow 判断该 key 是否可以放行。
// 返回 false 时 retryAfter 是建议等待的整秒数（至少 1 秒），用于 Retry-After 头。
func (l *Limiter) Allow(key string) (bool, int) {
	if l == nil || l.burst <= 0 {
		return true, 0
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweep(now)

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}

	// 按经过时间补充令牌，上限为桶容量。
	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens = math.Min(l.burst, b.tokens+elapsed*l.rate)
		b.last = now
	}

	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}

	// 距离攒够 1 个令牌还需要多久。
	need := (1 - b.tokens) / l.rate
	retry := int(math.Ceil(need))
	if retry < 1 {
		retry = 1
	}
	return false, retry
}

// sweep 定期丢弃长期闲置的桶，防止访客 id 无限增长撑爆内存。
// 闲置超过 10 分钟的桶，其令牌早已回满，丢弃与保留等价。
func (l *Limiter) sweep(now time.Time) {
	if now.Before(l.sweepAt) {
		return
	}
	l.sweepAt = now.Add(time.Minute)
	for k, b := range l.buckets {
		if now.Sub(b.last) > 10*time.Minute {
			delete(l.buckets, k)
		}
	}
}

// Len 返回当前活跃桶数量，仅供测试与观测使用。
func (l *Limiter) Len() int {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
