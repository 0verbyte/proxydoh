# `proxydoh`

Simple server that supports proxying DNS queries over HTTPS as specified in [DNS Queries over HTTPS (DoH)](https://tools.ietf.org/html/rfc8484)

DNS queries are sent as GET or POST requests using DNS wire format to HTTPS server providing DNS resolution. If the server provides `Cache-Control` header
`proxydoh` will use `max-age` value for it's own internal cache, this avoids additional network hops for DNS queries for the same host.

## Usage
```
% ./proxydoh -h
Usage of ./proxydoh:
  -debug
        Run the server in debug mode
  -dohserver string
        Set HTTPS server to receive DNS requests (default "https://cloudflare-dns.com/dns-query")
  -host string
        Server listen address (default "0.0.0.0")
  -httpMethod string
        Request method used when sending DNS query to HTTPS server (default "GET")
  -port int
        Server listen port (default 5553)
```

With `proxydoh` running, send it a DNS query.

```
dig @::1 -p 5553 google.com
```
