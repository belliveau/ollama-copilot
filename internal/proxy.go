package internal

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

func Proxy(port string, forward string) {
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
	var address string
	if strings.Contains(host, ":") {
		address = fmt.Sprintf("[%s]:%s", host, port)
	} else {
		address = fmt.Sprintf("%s:%s", host, port)
	}

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
