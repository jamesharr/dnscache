package dnscache

// This is a silly benchmark to see whether storing cacheEntry as a pointer or an object makes more sense.

// This can probably be removed at some poitn

import (
	"math"
	"testing"
)

type foo struct {
	count int
	err   error
}

// type cacheEntry2 struct {
// 	result        []string
// 	err           error
// 	pendingLookup bool
// 	lastRefresh   time.Time
// 	hits          int
// }

func cacheKey(iter int) string {
	bkt := int(float64(iter) / math.Log(float64(iter)))
	rv := [16]byte{}
	i := 0
	for ; i < len(rv) && bkt > 0; i++ {
		rv[i] = byte(bkt % 256)
		bkt = bkt / 256
	}
	return string(rv[:i])
}

func BenchmarkDirectMap(b *testing.B) {
	cache := make(map[string]cacheEntry)
	hits := make(map[string]int)
	for n := 0; n < b.N; n++ {
		k := cacheKey(n)
		entry, hit := cache[k]
		if hit {
			hits[k]++
		} else {
			entry.status = refreshQueued
			cache[k] = entry
		}
	}
	b.Logf("Cache size: %d\n", len(cache))
}

func BenchmarkPointerMap(b *testing.B) {
	cache := make(map[string]*cacheEntry)
	hits := make(map[string]int)
	for n := 0; n < b.N; n++ {
		k := cacheKey(n)
		entry := cache[k]
		if entry == nil {
			entry = &cacheEntry{}
			cache[k] = entry

			entry.status = refreshQueued
		} else {
			hits[k]++
		}
	}
	b.Logf("Cache size: %d\n", len(cache))
}
