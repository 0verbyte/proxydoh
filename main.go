package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/voidpirate/proxydoh/cache"
)

const (
	// BufferSize read buffer size for UDP socket
	BufferSize = 512

	// MIMEType included in the Content-Type request header to HTTPS server
	// providing DNS resolution
	MIMEType = "application/dns-message"
)

var (
	dohServer string

	host            string
	port            int
	httpMethod      string
	requestHandlers = map[string]func([]byte) (*http.Request, error){
		"GET":  createGETRequest,
		"POST": createPOSTRequest,
	}
	debug bool
)

func init() {
	flag.StringVar(&dohServer, "dohserver", "https://cloudflare-dns.com/dns-query", "Set HTTPS server to receive DNS requests")
	flag.StringVar(&host, "host", "0.0.0.0", "Server listen address")
	flag.IntVar(&port, "port", 5553, "Server listen port")
	flag.StringVar(&httpMethod, "httpMethod", "GET", "Request method used when sending DNS query to HTTPS server")
	flag.BoolVar(&debug, "debug", false, "Run the server in debug mode")

	flag.Parse()

	if debug {
		log.SetLevel(log.DebugLevel)
	}
}

func main() {
	httpMethod = strings.ToUpper(httpMethod)
	requestHandler, ok := requestHandlers[httpMethod]
	if !ok {
		log.WithFields(log.Fields{
			"method": httpMethod,
		}).Fatal("HTTP method not supported:")
	}

	log.WithFields(log.Fields{
		"method": httpMethod,
	}).Debug("Sending DNS over HTTPS")

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(host), Port: port})
	if err != nil {
		log.Fatalln(err)
	}

	log.WithFields(log.Fields{
		"host": host,
		"port": port,
	}).Info("Started UDP server:", net.JoinHostPort(host, strconv.Itoa(port)))

	for {
		buffer := make([]byte, BufferSize)
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Fatal("Error reading from socket:", err)
		}

		go handleConnection(conn, addr, buffer[:n], requestHandler)
	}
}

// Request headers sent to server providing DNS resolution over HTTPS
func addDNSRequestHeaders(dnsQuery []byte, req *http.Request) {
	req.Header.Add("Accept", MIMEType)
	req.Header.Add("Content-Type", MIMEType)
	req.Header.Add("Content-Length", strconv.Itoa(len(dnsQuery)))
}

// Create POST request to be sent to HTTPS server
func createPOSTRequest(dnsQuery []byte) (*http.Request, error) {
	body := bytes.NewBuffer(dnsQuery)
	req, err := http.NewRequest("POST", dohServer, body)
	if err != nil {
		return nil, err
	}

	addDNSRequestHeaders(dnsQuery, req)

	log.WithFields(log.Fields{
		"method": "POST",
	}).Debug("Created DNS HTTP request")

	return req, nil
}

// Create GET request to be sent to HTTP server
func createGETRequest(dnsQuery []byte) (*http.Request, error) {
	encodedQuery := base64.StdEncoding.EncodeToString(dnsQuery)
	encodedURL := fmt.Sprintf("%s?dns=%s", dohServer, encodedQuery)
	req, err := http.NewRequest("GET", encodedURL, nil)
	if err != nil {
		return nil, err
	}

	addDNSRequestHeaders(dnsQuery, req)

	log.WithFields(log.Fields{
		"method": "GET",
	}).Debug("Created DNS HTTP request")

	return req, nil
}

func handleConnection(conn *net.UDPConn, addr *net.UDPAddr, dnsQuery []byte, fn func([]byte) (*http.Request, error)) {
	client := &http.Client{}

	cacheReply, ok, _ := cache.Get(dnsQuery)
	if ok {
		_, err := conn.WriteToUDP(cacheReply, addr)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("Error writing to socket")
		}
		return
	}

	req, err := fn(dnsQuery)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Failed creating request")
	}

	resp, err := client.Do(req)
	if err != nil {
		log.WithFields(log.Fields{
			"error":    err,
			"upstream": dohServer,
		}).Error("Sending DNS request to upstream HTTP DNS server")
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.WithFields(log.Fields{
			"code": resp.StatusCode,
		}).Error("Invalid HTTP response from upstream")
		return
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Error reading response body from upstream")
	}

	cache.Add(dnsQuery, body, resp.Header)

	n, err := conn.WriteToUDP(body, addr)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Error writing to client UDP connection")
		return
	}

	log.WithFields(log.Fields{
		"totalBytes": n,
	}).Debug("Replied to DNS query")
}
