// cache.go 是服务端内存 LRU 翻译缓存（FR-1101，DR-5）。
//
// @file internal/gateway/cache.go
// 文件职责：以 hash(模型+语言对+风格+术语指纹+段落文本) 为键缓存译文；
// 命中不调上游、不计配额；默认 5000 条 / 32 MiB，TTL 24h，重启清零，
// FR_CACHE_ENABLED=false 可整体关闭。缓存仅在内存，不构成持久化存储（隐私红线不受影响）。
//
// 主要内容：Cache、NewCache、Get、Put、Len。
//
// 模块边界：只依赖标准库；被 service.translate 使用；键的构造在调用方（避免本包认识业务字段）。
package gateway

import (
	"container/list"
	"sync"
	"time"
)

// entry 是缓存条目。
type entry struct {
	key       string
	value     string
	size      int
	expiresAt time.Time
}

// Cache 是 LRU + TTL + 双上限（条数/字节）的内存缓存。
type Cache struct {
	mu       sync.Mutex
	items    map[string]*list.Element
	order    *list.List // 前端=最近使用，后端=最久未用
	maxItems int
	maxBytes int
	bytes    int
	ttl      time.Duration
	now      func() time.Time
}

// 缺省上限（FR-1101）。
const (
	DefaultMaxItems = 5000
	DefaultMaxBytes = 32 << 20
	DefaultTTL      = 24 * time.Hour
)

// NewCache 构造缓存。maxItems/maxBytes<=0 时用默认值。
func NewCache(maxItems, maxBytes int, ttl time.Duration) *Cache {
	if maxItems <= 0 {
		maxItems = DefaultMaxItems
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Cache{
		items:    make(map[string]*list.Element),
		order:    list.New(),
		maxItems: maxItems,
		maxBytes: maxBytes,
		ttl:      ttl,
		now:      time.Now,
	}
}

// Get 返回缓存命中值。过期或不存在返回 false。
// 命中会把条目移到队首（LRU 语义）。
func (c *Cache) Get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return "", false
	}
	e := el.Value.(*entry)
	if c.now().After(e.expiresAt) {
		c.remove(el)
		return "", false
	}
	c.order.MoveToFront(el)
	return e.value, true
}

// Put 写入缓存，并按条数/字节上限从队尾淘汰。
// 超大条目（单条超过字节上限）不缓存：留着只会立刻被淘汰，还挤掉别的条目。
func (c *Cache) Put(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	size := len(key) + len(value)
	if size > c.maxBytes {
		return
	}
	if el, ok := c.items[key]; ok {
		e := el.Value.(*entry)
		c.bytes -= e.size
		e.value = value
		e.size = size
		e.expiresAt = c.now().Add(c.ttl)
		c.bytes += size
		c.order.MoveToFront(el)
		return
	}

	el := c.order.PushFront(&entry{key: key, value: value, size: size, expiresAt: c.now().Add(c.ttl)})
	c.items[key] = el
	c.bytes += size

	for c.order.Len() > c.maxItems || c.bytes > c.maxBytes {
		back := c.order.Back()
		if back == nil {
			break
		}
		c.remove(back)
	}
}

// Len 返回当前条数，供测试与观测。
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// remove 删除一个元素（须持锁）。
func (c *Cache) remove(el *list.Element) {
	e := el.Value.(*entry)
	c.order.Remove(el)
	delete(c.items, e.key)
	c.bytes -= e.size
}
