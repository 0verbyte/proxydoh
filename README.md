# `proxydoh`

Simple server that supports proxying DNS requests over HTTPS as specified in [DNS Queries over HTTPS (DoH) draft-ietf-doh-dns-over-https-14](https://tools.ietf.org/html/draft-ietf-doh-dns-over-https-14)

DNS requests are sent as GET/POST requests using DNS wire format to HTTPS server providing DNS resolution.


## Usage
```
% ./proxydoh -h
Usage of ./proxydoh:
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