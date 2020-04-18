package dnscache

import (
	"sync"
	"time"
)

// DefaultResolverCount is the default number of resolvers used in a cache
const DefaultResolverCount int = 20

// DefaultPurgeEvery is the default idle time of the purge process
const DefaultPurgeEvery time.Duration = time.Minute

// DefaultTTL is the default TTL of caches and can be overridden
const DefaultTTL time.Duration = time.Hour

// CacheResolver User Interface
type CacheResolver struct {
	// Settings should not be changed after CacheResolver is started
	ResolverCount int
	PurgeEvery    time.Duration
	TTL           time.Duration

	// Cache state
	started bool

	// Cache
	cacheLock sync.RWMutex
	cache     map[string]cacheEntry

	// send a request into the queue
	queryQueue chan string

	// Stats
	statsLock  sync.RWMutex
	hitsByName map[string]int
	hits       int
	misses     int
	queueSize  chan int // Queue manager sends the size of the queue
}

type cacheEntry struct {
	result        []string
	err           error
	pendingLookup bool
	lastRefresh   time.Time
}

// CacheStats docs TODO
type CacheStats struct {
	Hits      int
	Misses    int
	QueueSize int
	CacheSize int
}

// Start resolver
func (cr *CacheResolver) Start() {
	// Check defaults
	if cr.TTL == time.Duration(0) {
		cr.TTL = DefaultTTL
	}
	if cr.PurgeEvery == time.Duration(0) {
		cr.PurgeEvery = DefaultPurgeEvery
	}
	if cr.ResolverCount == 0 {
		cr.ResolverCount = DefaultResolverCount
	}

	// Initialize other fields
	cr.cache = make(map[string]cacheEntry)
	cr.hitsByName = make(map[string]int)
	cr.queueSize = make(chan int)

	// Start up GoRoutines
	cr.started = true
	go cr.cacheManager()
	go cr.cachePurger()
	for i := 0; i < cr.ResolverCount; i++ {
		go cr.cacheResolver()
	}
}

// LookupAddr TODO
func (cr *CacheResolver) LookupAddr(addr string) (names []string, err error) {
	return
}

// CacheStats TODO
func (cr *CacheResolver) CacheStats() CacheStats {
	rv := CacheStats{}
	cr.cacheLock.RLock()
	defer cr.cacheLock.RUnlock()
	return rv
}

// Cache queue manager
func (cr *CacheResolver) cacheManager() {

}

// Cache expunger
func (cr *CacheResolver) cachePurger() {

}

// Cache resolver goroutine
func (cr *CacheResolver) cacheResolver() {

}
