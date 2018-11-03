package cache

import (
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// CacheControl response header
	CacheControl = "Cache-Control"

	// CacheValueDelimiter Cache-Control header value is delimited by "=" (e.g. max-age=177)
	CacheValueDelimiter = "="

	// KeyCheckSumSize The size of a SHA256 checksum in bytes.
	KeyCheckSumSize = 32

	// DNSHeaderSize size of DNS header.
	DNSHeaderSize = 8
)

var cache = struct {
	sync.Mutex
	values map[[KeyCheckSumSize]byte]DNSResponse
}{
	values: make(map[[KeyCheckSumSize]byte]DNSResponse),
}

// DNSResponse stores DNS reply in wire-format and TTL
type DNSResponse struct {
	Reply []byte
	TTL   int64
}

// Get check if dnsquery is still in cache and hasn't expired
func Get(dnsQuery []byte) ([]byte, bool, error) {
	cache.Lock()
	cacheKey := sha256.Sum256(dnsQuery[DNSHeaderSize:])
	c, ok := cache.values[cacheKey]
	defer cache.Unlock()
	if !ok {
		return nil, false, nil
	}

	if c.TTL <= time.Now().Unix() {
		log.Println("Cache expired...")
		delete(cache.values, cacheKey)
		return nil, false, nil
	}

	// Add the current ID to cached DNS query reply
	reply := append(dnsQuery[:DNSHeaderSize], c.Reply[DNSHeaderSize:]...)

	log.Println("Found DNS query in cache, ttl:", c.TTL-time.Now().Unix())

	return reply, true, nil
}

// Add caches DNS query reply
func Add(dnsQuery []byte, dnsReply []byte, headers http.Header) error {
	dnsResponse := DNSResponse{
		Reply: dnsReply,
	}

	ttlHeader := headers.Get(CacheControl)
	if ttlHeader == "" {
		return fmt.Errorf("%s header does not exist or is empty", CacheControl)
	}

	value := strings.Split(ttlHeader, CacheValueDelimiter)
	ttl, err := strconv.Atoi(strings.TrimSpace(value[1]))
	if err != nil {
		return err
	}
	dnsResponse.TTL = int64(ttl) + time.Now().Unix()

	cacheKey := sha256.Sum256(dnsQuery[DNSHeaderSize:])

	cache.Lock()
	cache.values[cacheKey] = dnsResponse
	defer cache.Unlock()
	log.Printf("Saved DNS query to cache: %x\n", cacheKey)

	return nil
}

func logSavedCache(key [KeyCheckSumSize]byte) {
	cache.Lock()
	v := cache.values[key]
	defer cache.Unlock()
	fmt.Println("Cache TTL", v.TTL)
}
