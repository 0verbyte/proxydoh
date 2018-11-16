package cache

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	// CacheControl response header
	CacheControl = "Cache-Control"

	// CacheValueDelimiter Cache-Control header value is delimited by "=" (e.g. max-age=177)
	CacheValueDelimiter = "="

	// KeyCheckSumSize The size of a SHA256 checksum in bytes.
	KeyCheckSumSize = 32

	// DNSHeaderID A 16 bit identifier assigned by the program that
	// generates any kind of query. This identifier is copied the corresponding
	// reply and can be used by the requester to match up replies to outstanding queries.
	DNSHeaderID = 2
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
func Get(dnsQuery []byte) []byte {
	cache.Lock()
	cacheKey := sha256.Sum256(dnsQuery[DNSHeaderID:])
	c, ok := cache.values[cacheKey]
	defer cache.Unlock()
	if !ok {
		return nil
	}

	if c.TTL <= time.Now().Unix() {
		log.WithFields(log.Fields{
			"key": fmt.Sprintf("%x", cacheKey),
		}).Debug("Cache expired")
		delete(cache.values, cacheKey)
		return nil
	}

	log.WithFields(log.Fields{
		"ttl": c.TTL - time.Now().Unix(),
		"key": fmt.Sprintf("%x", cacheKey),
	}).Debug("Cache hit for dns query")

	reply := append(dnsQuery[:DNSHeaderID], c.Reply[DNSHeaderID:]...)
	return reply
}

// Add caches DNS query reply
func Add(dnsQuery []byte, dnsReply []byte, headers http.Header) {
	dnsResponse := DNSResponse{
		Reply: dnsReply,
	}

	ttlHeader := headers.Get(CacheControl)
	if ttlHeader == "" {
		log.WithFields(log.Fields{
			"header": CacheControl,
		}).Warn("header does not exist or is empty")
		return
	}

	value := strings.Split(ttlHeader, CacheValueDelimiter)
	ttl, err := strconv.Atoi(strings.TrimSpace(value[1]))
	if err != nil {
		log.WithFields(log.Fields{
			"header": CacheControl,
			"field":  "max-age",
		}).Warn("Unable to convert header to int")
		return
	}
	dnsResponse.TTL = int64(ttl) + time.Now().Unix()

	cacheKey := sha256.Sum256(dnsQuery[DNSHeaderID:])

	cache.Lock()
	cache.values[cacheKey] = dnsResponse
	defer cache.Unlock()
	log.WithFields(log.Fields{
		"key": fmt.Sprintf("%x", cacheKey),
	}).Debug("Saved DNS query to cache")
}
