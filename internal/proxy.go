package internal

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

var enableMITM = flag.Bool("mitm", false, "Enable MITM mode for HTTPS interception")

func init() {
	flag.Parse()
}

func Proxy(port string, forward string) {
	certPath := "server.crt"
	keyPath := "server.key"
	if err := ensureCertExists(certPath, keyPath); err != nil {
		log.Fatalf("failed to ensure certificate exists: %v", err)
	}

	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatalf("failed to accept: %v", err)
		}

		go handle(conn, forward)
	}
}

var hosts = []string{
	"api.githubcopilot.com",
	"api.github.com",
	"copilot-proxy.githubusercontent.com",
	"proxy.individual.githubcopilot.com",
	"proxy.business.githubcopilot.com",
	"copilot-telemetry.githubusercontent.com",
}

func handle(conn net.Conn, forward string) {
	req, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		conn.Close()
		log.Printf("failed to read request: %v", err)
		return
	}

	host := req.URL.Hostname()
	port := req.URL.Port()
	if port == "" {
		// Default to 443 for HTTPS CONNECT requests
		if req.Method == http.MethodConnect {
			port = "443"
		} else {
			port = "80"
		}
	}
	address := net.JoinHostPort(host, port)

	log.Printf("CONNECT request for host: %s (address: %s)", host, address)
	knownHost := false
	for _, h := range hosts {
		if strings.Contains(host, h) {
			// Forward to local Ollama instance
			address = "localhost" + forward
			knownHost = true
			break
		}
	}

	// Proxy Copilot model list endpoint
	if req.Method == http.MethodGet && req.URL.Path == "/v1/models" {
		// Forward to local /models endpoint
		resp, err := http.Get("http://localhost" + forward + "/models")
		if err != nil {
			conn.Close()
			log.Printf("failed to get models: %v", err)
			return
		}
		defer resp.Body.Close()
		// Write HTTP response header
		fmt.Fprintf(conn, "HTTP/1.1 %d %s\r\n", resp.StatusCode, http.StatusText(resp.StatusCode))
		for k, v := range resp.Header {
			for _, vv := range v {
				fmt.Fprintf(conn, "%s: %s\r\n", k, vv)
			}
		}
		fmt.Fprint(conn, "\r\n")
		io.Copy(conn, resp.Body)
		conn.Close()
		return
	}

	if req.Method == http.MethodConnect && *enableMITM {
		log.Printf("Intercepting HTTPS traffic for MITM: %s", address)
		mitmHandler(conn, req, "server.crt", "server.key")
		return
	}

	if req.Method != http.MethodConnect {
		// Catch-all for unknown HTTP requests
		log.Printf("catch-all: %s %s", req.Method, req.URL.Path)
		fmt.Fprintf(conn, "HTTP/1.1 404 Not Found\r\nContent-Type: text/plain\r\n\r\nEndpoint not handled by proxy\n")
		conn.Close()
		return
	}

	if !knownHost {
		log.Printf("unknown host in CONNECT: %s (address: %s)", host, address)
		conn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain\r\n\r\nUnknown host, not forwarding.\n"))
		conn.Close()
		return
	}

	client, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		conn.Close()
		log.Printf("failed to dial: %v", err)
		_, err = conn.Write([]byte("HTTP/1.1 500 Internal Server Error\r\n\r\n"))
		if err != nil {
			log.Printf("failed to write response: %v", err)
		}
		return
	}

	_, err = conn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
	if err != nil {
		conn.Close()
		log.Printf("failed to write response: %v", err)
		return
	}

	go transfer(client, conn)
	go transfer(conn, client)
}

func mitmHandler(conn net.Conn, req *http.Request, certPath, keyPath string) {
	// Load the server certificate and key
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		log.Printf("failed to load certificate: %v", err)
		conn.Close()
		return
	}

	// Create a TLS configuration using the certificate
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	// Establish a TLS connection with the client
	tlsConn := tls.Server(conn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		log.Printf("TLS handshake failed: %v", err)
		conn.Close()
		return
	}

	// Forward the request to the target server
	targetConn, err := net.Dial("tcp", req.Host)
	if err != nil {
		log.Printf("failed to connect to target: %v", err)
		conn.Close()
		return
	}

	// Establish a TLS connection with the target server
	tlsTargetConn := tls.Client(targetConn, &tls.Config{
		InsecureSkipVerify: true, // Skip verification for demonstration purposes
	})
	if err := tlsTargetConn.Handshake(); err != nil {
		log.Printf("TLS handshake with target failed: %v", err)
		conn.Close()
		targetConn.Close()
		return
	}

	// Relay traffic between the client and the target server
	go transfer(tlsConn, tlsTargetConn)
	go transfer(tlsTargetConn, tlsConn)
}

func transfer(w io.WriteCloser, r io.ReadCloser) {
	defer w.Close()
	defer r.Close()
	_, err := io.Copy(w, r)
	if errors.Is(err, net.ErrClosed) {
		return
	}

	if err == net.ErrClosed {
		return
	}

	if err != nil {
		log.Printf("failed to transfer: %v", err)
	}
}

func generateSelfSignedCert(certPath, keyPath string) error {
	// Generate a private key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Local Proxy"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %v", err)
	}

	certFile, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("failed to create cert file: %v", err)
	}
	defer certFile.Close()

	pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyFile, err := os.Create(keyPath)
	if err != nil {
		return fmt.Errorf("failed to create key file: %v", err)
	}
	defer keyFile.Close()

	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %v", err)
	}

	pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})

	return nil
}

func ensureCertExists(certPath, keyPath string) error {
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		log.Printf("Certificate not found, generating a new one at %s", certPath)
		return generateSelfSignedCert(certPath, keyPath)
	}
	return nil
}
