`dnscache` is an experimental concurrent DNS resolver with an in-memory cache. Its targetted use-case is
for doing network analytics where a large number of lookups are expected and teh first
attempted lookup may return an NXDomain response.

Some key characteristics
* Uses concurrent parallel resolvers that run in the background.
* The first attempt to perform a reverse lookup will fail the first time,
  but will queue a lookup to later populate the cache.
* The cache will track DNS lookups and automatically refresh entries that have been used
  when the TTL expires, and expunge records that have not been used.

Possible TODO:
* Blocking Lookups - wait for an entry to exist in the cache.
* Allow the use of a custom resolver. This should also help with testing
* Add other DNS lookup types (if this experiment proves useful)
* Some form of cache size limit w/ early expiration. IE limit to 2M entries, purge LRU.