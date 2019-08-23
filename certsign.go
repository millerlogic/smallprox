// Code pulled from github.com/elazarl/goproxy
// See license https://github.com/elazarl/goproxy/blob/master/LICENSE

package smallprox

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/exp/errors"
	"golang.org/x/exp/errors/fmt"
)

type myCertEnt struct {
	cert *tls.Certificate
	t    time.Time
}

type myCertStore struct {
	m  map[string]myCertEnt
	mx sync.Mutex
}

func (store *myCertStore) Fetch(hostname string, gen func() (*tls.Certificate, error)) (*tls.Certificate, error) {
	store.mx.Lock()
	defer store.mx.Unlock()
	lhostname := strings.ToLower(hostname)
	if e, ok := store.m[lhostname]; ok {
		return e.cert, nil
	}
	now := time.Now()
	const maxEnts = 50
	if len(store.m) >= maxEnts {
		old := now.Add(-time.Hour)
		for k, v := range store.m {
			if v.t.Before(old) {
				delete(store.m, k)
			}
		}
		for len(store.m) >= maxEnts {
			for k := range store.m {
				delete(store.m, k)
				break
			}
		}
	}
	cert, err := gen()
	if err != nil {
		return nil, err
	}
	store.m[lhostname] = myCertEnt{cert: cert, t: now}
	return cert, nil
}

func newCertStore() *myCertStore {
	return &myCertStore{m: make(map[string]myCertEnt)}
}

func signHost(ca tls.Certificate, hosts []string) (cert *tls.Certificate, err error) {
	var x509ca *x509.Certificate

	if x509ca, err = x509.ParseCertificate(ca.Certificate[0]); err != nil {
		return
	}
	start := time.Unix(0, 0)
	end, err := time.Parse("2006-01-02", "2049-12-31")
	if err != nil {
		panic(err)
	}
	hash := make([]byte, 20)
	_, err = rand.Read(hash)
	if err != nil {
		return nil, err
	}
	serial := new(big.Int)
	serial.SetBytes(hash)
	template := x509.Certificate{
		SerialNumber: serial,
		Issuer:       x509ca.Subject,
		Subject: pkix.Name{
			Organization: []string{"Internet Widgits Pty Ltd"},
		},
		NotBefore: start,
		NotAfter:  end,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
			template.Subject.CommonName = h
		}
	}

	var csprng CounterEncryptorRand
	if csprng, err = NewCounterEncryptorRandFromKey(ca.PrivateKey, hash); err != nil {
		return
	}

	var certpriv crypto.Signer
	switch ca.PrivateKey.(type) {
	case *rsa.PrivateKey:
		if certpriv, err = rsa.GenerateKey(&csprng, 2048); err != nil {
			return
		}
	case *ecdsa.PrivateKey:
		if certpriv, err = ecdsa.GenerateKey(elliptic.P256(), &csprng); err != nil {
			return
		}
	default:
		err = fmt.Errorf("unsupported key type %T", ca.PrivateKey)
	}

	var derBytes []byte
	if derBytes, err = x509.CreateCertificate(&csprng, &template, x509ca, certpriv.Public(), ca.PrivateKey); err != nil {
		return
	}
	return &tls.Certificate{
		Certificate: [][]byte{derBytes, ca.Certificate[0]},
		PrivateKey:  certpriv,
	}, nil
}

type CounterEncryptorRand struct {
	cipher  cipher.Block
	counter []byte
	rand    []byte
	ix      int
}

func NewCounterEncryptorRandFromKey(key interface{}, seed []byte) (r CounterEncryptorRand, err error) {
	var keyBytes []byte
	switch key := key.(type) {
	case *rsa.PrivateKey:
		keyBytes = x509.MarshalPKCS1PrivateKey(key)
	case *ecdsa.PrivateKey:
		if keyBytes, err = x509.MarshalECPrivateKey(key); err != nil {
			return
		}
	default:
		err = errors.New("only RSA and ECDSA keys supported")
		return
	}
	h := sha256.New()
	if r.cipher, err = aes.NewCipher(h.Sum(keyBytes)[:aes.BlockSize]); err != nil {
		return
	}
	r.counter = make([]byte, r.cipher.BlockSize())
	if seed != nil {
		copy(r.counter, h.Sum(seed)[:r.cipher.BlockSize()])
	}
	r.rand = make([]byte, r.cipher.BlockSize())
	r.ix = len(r.rand)
	return
}

func (c *CounterEncryptorRand) Seed(b []byte) {
	if len(b) != len(c.counter) {
		panic("SetCounter: wrong counter size")
	}
	copy(c.counter, b)
}

func (c *CounterEncryptorRand) refill() {
	c.cipher.Encrypt(c.rand, c.counter)
	for i := 0; i < len(c.counter); i++ {
		if c.counter[i]++; c.counter[i] != 0 {
			break
		}
	}
	c.ix = 0
}

func (c *CounterEncryptorRand) Read(b []byte) (n int, err error) {
	if c.ix == len(c.rand) {
		c.refill()
	}
	if n = len(c.rand) - c.ix; n > len(b) {
		n = len(b)
	}
	copy(b, c.rand[c.ix:c.ix+n])
	c.ix += n
	return
}
