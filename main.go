package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
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

	host       string
	port       int
	httpMethod string
)

func init() {
	flag.StringVar(&dohServer, "dohserver", "https://cloudflare-dns.com/dns-query", "Set HTTPS server to receive DNS requests")
	flag.StringVar(&host, "host", "0.0.0.0", "Server listen address")
	flag.IntVar(&port, "port", 5553, "Server listen port")
	flag.StringVar(&httpMethod, "httpMethod", "GET", "Request method used when sending DNS query to HTTPS server")

	flag.Parse()

	httpMethod = strings.ToUpper(httpMethod)
}

func main() {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(host), Port: port})
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("Started UDP server:", net.JoinHostPort(host, strconv.Itoa(port)))

	for {
		buffer := make([]byte, BufferSize)
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Println("Error reading from socket:", err)
		}

		go handleConnection(conn, addr, buffer[:n])
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

	log.Println("Created POST DNS HTTP request")

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

	log.Println("Created GET DNS HTTP request")

	return req, nil
}

func handleConnection(conn *net.UDPConn, addr *net.UDPAddr, dnsQuery []byte) {
	client := &http.Client{}

	var req *http.Request
	var err error
	if httpMethod == "GET" {
		req, err = createGETRequest(dnsQuery)
	} else if httpMethod == "POST" {
		req, err = createPOSTRequest(dnsQuery)
	} else {
		log.Println("HTTP method not implemented:", httpMethod)
		return
	}

	if err != nil {
		log.Println("Failed creating request:", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error sending request:", err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Invalid HTTP response (status code: %d)\n", resp.StatusCode)
		return
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading response body:", err)
	}

	n, err := conn.WriteToUDP(body, addr)
	if err != nil {
		log.Println("Error writing to UDP socket:", err)
		return
	}

	log.Printf("Replied to DNS query with DNS response (%d bytes sent)", n)
}
