# smallprox

Smallprox is small HTTP proxy with features such as HTTPS MITM and content manipulation, written in Go.

## Usage
```
Usage of smallprox:
  -addr value
    	Proxy listen address(es)
  -auth string
    	Proxy authentication, username:password
  -blockHostsFile value
    	Block the hosts found in this file(s), one per line or in /etc/hosts format
  -cacert string
    	CA certificate file for HTTPS MITM
  -cakey string
    	CA private key file for HTTPS MITM
  -compress
    	Compress highly compressable content * (default true)
  -connectMITM
    	Enable man-in-the-middle for HTTP CONNECT connections (default true)
  -httpsMITM
    	Enable man-in-the-middle for HTTPS CONNECT connections
  -insecure
    	TLS config InsecureSkipVerify
  -limitContent value
    	Limit content to minimize excessive memory usage * (default 100MiB)
  -noscript
    	Remove JavaScript from HTML content *
  -shrinkImages
    	Make images/pictures smaller *
  -v	Verbose output
* only applies to CONNECT if MITM enabled
```

## Docker
```
docker build --tag millerlogic/smallprox .

docker run --rm -it -p 127.0.0.1:8080:8080 --user=nobody:nobody --init millerlogic/smallprox
```
