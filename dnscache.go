package dnscache

import (
	"fmt"
	"net"
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
	Workers    int
	PurgeEvery time.Duration
	TTL        time.Duration

	// Cache state
	started bool

	// Cache
	lock  sync.RWMutex
	cache map[string]*cacheEntry

	requestChan  chan string // client -> queue
	resolverChan chan string // queue -> resolver worker

	// Stats
	queueSize         chan int // Queue manager sends the size of the queue
	lastPurgeDuration time.Duration
	hits              int
	misses            int
}

type cacheEntry struct {
	names       []string
	err         error
	status      cacheEntryStatus
	requests    int
	lastRefresh time.Time
}

type cacheEntryStatus uint8

const (
	nonePending cacheEntryStatus = iota
	refreshQueued
	refreshInProgress
)

// CacheStats docs TODO
type CacheStats struct {
	Hits              int
	Misses            int
	QueueSize         int
	CacheSize         int
	LastPurgeDuration time.Duration
}

// Start resolver. Attempting to perform a lookup prior to starting the resolver
// will result in a panic.
func (cr *CacheResolver) Start() {
	// Check defaults
	if cr.TTL == time.Duration(0) {
		cr.TTL = DefaultTTL
	}
	if cr.PurgeEvery == time.Duration(0) {
		cr.PurgeEvery = DefaultPurgeEvery
	}
	if cr.Workers == 0 {
		cr.Workers = DefaultResolverCount
	}

	// Initialize fields
	cr.cache = make(map[string]*cacheEntry)
	cr.queueSize = make(chan int)
	cr.requestChan = make(chan string)
	cr.resolverChan = make(chan string)

	// Start up GoRoutines
	cr.started = true
	go cr.queueManager()
	go cr.cachePurger()
	for i := 0; i < cr.Workers; i++ {
		go cr.cacheResolver()
	}
}

// ErrLookupPending is a well known error for when the cache is pending being populated
var ErrLookupPending error = fmt.Errorf("Lookup pending")

// LookupAddr looks up addr in the cache and returns the cached results from net.LookupAddr including any error.
// If no entry exists in the cache then `nil, ErrLookupPending` is returned and a lookup is enqueued.
func (cr *CacheResolver) LookupAddr(addr string) (names []string, err error) {
	cr.lock.Lock()
	defer cr.lock.Unlock()

	// Get cache entry
	ent := cr.cache[addr]
	if ent == nil {
		// Create a new entry
		ent = &cacheEntry{
			status: refreshQueued,
			err:    ErrLookupPending,
		}
		cr.cache[addr] = ent

		// Submit queue lookup
		cr.requestChan <- addr
	}

	// Increment hit counters
	if ent.status == nonePending {
		cr.hits++
	} else {
		cr.misses++
	}

	return ent.names, ent.err
}

// CacheStats TODO
func (cr *CacheResolver) CacheStats() CacheStats {
	rv := CacheStats{}
	rv.QueueSize = <-cr.queueSize
	cr.lock.RLock()
	defer cr.lock.RUnlock()
	rv.Hits = cr.hits
	rv.Misses = cr.misses
	rv.LastPurgeDuration = cr.lastPurgeDuration
	rv.CacheSize = len(cr.cache)
	return rv
}

// Cache queue manager
func (cr *CacheResolver) queueManager() {
	queue := []string{}

	// Loop infinitely
	for {
		// rc will be left nil if we don't have anything to send
		var rc chan string = nil
		var nextItem string
		if len(queue) > 0 {
			rc = cr.resolverChan
			nextItem = queue[0]
		}

		// Wait for next communication event
		select {
		case cr.queueSize <- len(queue): // Request for queue size
		case rc <- nextItem: // Worker requesting an item
			queue = queue[1:]
		case newReq := <-cr.requestChan: // New request inbound
			queue = append(queue, newReq)
		}
	}
}

// Cache expunger
func (cr *CacheResolver) cachePurger() {
	// Loop indefinitely
	for {
		// Wait for specified period of time
		time.Sleep(cr.PurgeEvery)

		purgeStart := time.Now()
		purgeCutoff := purgeStart.Add(-cr.TTL)
		cacheExpire := []string{}
		cacheRefresh := []string{}

		// cacheLock scope
		func() {
			cr.lock.Lock()
			defer cr.lock.Unlock()

			// Check status of each entry
			//  Skip if the entry is pending a refresh in some respect
			//  Skip if the entry is still OK
			//  Purge if the entry hasn't hit a certain request threshold
			//  Refresh if the entry has hit a certain threshold
			for addr, cacheEntry := range cr.cache {
				if cacheEntry.status == nonePending && cacheEntry.lastRefresh.Before(purgeCutoff) {
					if cacheEntry.requests > 1 {
						cacheRefresh = append(cacheRefresh, addr)
					} else {
						cacheExpire = append(cacheExpire, addr)
						cacheEntry.status = refreshQueued
					}
				}
			}

			// Expunge expired
			for _, addr := range cacheExpire {
				delete(cr.cache, addr)
			}
		}()

		// Send in refresh request outside of the lock
		for _, addr := range cacheRefresh {
			cr.requestChan <- addr
		}

		// Update the purgeDuration
		purgeEnd := time.Now()
		func() {
			cr.lock.Lock()
			defer cr.lock.Unlock()
			cr.lastPurgeDuration = purgeEnd.Sub(purgeStart)
		}()
	}
}

// Cache resolver goroutine
func (cr *CacheResolver) cacheResolver() {
	for {
		addr := <-cr.resolverChan
		skipLookup := true

		// Check cache entry status, and update
		func() {
			cr.lock.Lock()
			defer cr.lock.Unlock()

			// Snag from cache
			ent := cr.cache[addr]
			if ent == nil {
				ent = &cacheEntry{}
				cr.cache[addr] = ent
			}

			// Check and update cacheEntry status
			if ent.status == refreshQueued {
				ent.status = refreshInProgress
				skipLookup = false
			}
			// ent.status == refreshInProgress: another worker is working on it
			// ent.status == nonePending: someone else refreshed it
		}()

		// Skip if we determine we don't need to perform a lookup
		if skipLookup {
			continue
		}

		// Perform DNS lookup
		names, err := net.LookupAddr(addr)

		// Update cache
		func() {
			cr.lock.Lock()
			defer cr.lock.Unlock()

			// Snag from cache
			ent := cr.cache[addr]
			if ent == nil {
				ent = &cacheEntry{}
				cr.cache[addr] = ent
			}

			// Update cache entry
			ent.names = names
			ent.err = err
			ent.lastRefresh = time.Now()
			ent.status = nonePending
			ent.requests = 0
		}()
	}
}
