package dnscache

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var nilResult = []string(nil)

func TestResolverSimple(t *testing.T) {
	var cache CacheResolver
	cache.PurgeEvery = time.Millisecond * 45
	cache.Workers = 5
	cache.Start()

	// First lookup attempt will fail with a pending message
	names, err := cache.LookupAddr("1.1.1.1")
	assert.Equal(t, ErrLookupPending, err)
	assert.Equal(t, nilResult, names)

	names, err = cache.LookupAddr("1.1.1.2")
	assert.Equal(t, ErrLookupPending, err)
	assert.Equal(t, nilResult, names)

	// Wait for resolver(s) to do their thing
	time.Sleep(time.Millisecond * 250)

	// Second lookup will come back
	names, err = cache.LookupAddr("1.1.1.1")
	assert.NoError(t, err)
	assert.Equal(t, []string{"one.one.one.one."}, names)

	names, err = cache.LookupAddr("1.1.1.2")
	assert.Error(t, err)
	dnsErr := err.(*net.DNSError)
	assert.Equal(t, true, dnsErr.IsNotFound)
	assert.Equal(t, nilResult, names)

	// Check stats for correct reporting
	stats := cache.CacheStats()
	t.Logf("%v\n", stats)
	assert.Equal(t, 0, stats.QueueSize)
	assert.Equal(t, 2, stats.Hits)
	assert.Equal(t, 2, stats.Misses)
	assert.Equal(t, 2, stats.CacheSize)
	assert.LessOrEqual(t, int64(time.Nanosecond), int64(stats.LastPurgeDuration), "Purge not run")
}

// Ensure QueueSize is getting reported as something
func TestResolverStatsQueueSize(t *testing.T) {
	var cache CacheResolver
	cache.Workers = 1
	cache.Start()
	cache.LookupAddr("1.2.3.4")
	cache.LookupAddr("2.3.4.5")
	cache.LookupAddr("3.4.5.6")
	cache.LookupAddr("4.5.6.7")
	cache.LookupAddr("7.8.9.0")
	cache.LookupAddr("8.9.0.1")
	cache.LookupAddr("9.0.1.2")
	cache.LookupAddr("0.1.2.3")
	stats := cache.CacheStats()
	assert.LessOrEqual(t, 1, stats.QueueSize)
}
