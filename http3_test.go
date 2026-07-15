package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/quic-go/quic-go/http3"
)

// TestH3RoundTripLoopback verifies the HTTP/3 wiring (custom Dial, UDP-socket
// lifecycle, round-trip, status check) end-to-end against a local quic-go h3
// server over loopback UDP. This needs NO external UDP/443 egress — important
// because many networks (including some CI/dev hosts) block outbound QUIC, which
// would make a live speed.cloudflare.com probe falsely look broken.
func TestH3RoundTripLoopback(t *testing.T) {
	tlsConf := selfSignedTLS(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/cdn-cgi/trace", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("h=h3test\ncolo=TST\n"))
	})

	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	defer udpConn.Close()

	srv := &http3.Server{Handler: mux, TLSConfig: tlsConf}
	go srv.Serve(udpConn)
	defer srv.Close()

	endpoint := udpConn.LocalAddr().String()

	// Give the server a moment to start serving on the socket.
	time.Sleep(100 * time.Millisecond)

	ok := h3RoundTrip(context.Background(), endpoint, "h3test.local", "/cdn-cgi/trace", 5*time.Second, true)
	if !ok {
		t.Fatalf("h3RoundTrip(%s) = false, want true", endpoint)
	}

	// A wrong path (404) must read as not-reachable.
	if h3RoundTrip(context.Background(), endpoint, "h3test.local", "/nope", 5*time.Second, true) {
		t.Errorf("expected 404 path to report false")
	}
}

func TestApplyH3(t *testing.T) {
	results := []CleanIPResult{
		{Endpoint: "1.1.1.1:443", Success: true},
		{Endpoint: "1.0.0.1:443", Success: true},
		{Endpoint: "[2606:4700::1]:443", Success: true},
		{Endpoint: "8.8.8.8:443", Success: false},
		{Endpoint: "not-a-hostport", Success: true}, // ipOnly returns ""
	}
	h3Map := map[string]bool{
		"1.1.1.1":      true,
		"2606:4700::1": true,
		// 1.0.0.1 absent => stays false
		// 8.8.8.8 true would still mark H3 even if Success=false
		"8.8.8.8": true,
	}

	applyH3(results, h3Map)

	if !results[0].H3 {
		t.Errorf("1.1.1.1 should be H3")
	}
	if results[1].H3 {
		t.Errorf("1.0.0.1 should not be H3 (absent from map)")
	}
	if !results[2].H3 {
		t.Errorf("IPv6 2606:4700::1 should be H3")
	}
	if !results[3].H3 {
		t.Errorf("8.8.8.8 is in map so H3 must be set regardless of Success")
	}
	if results[4].H3 {
		t.Errorf("malformed endpoint must not match empty-key lookup")
	}

	// empty map is a no-op (does not clear existing flags either — it returns early)
	pre := []CleanIPResult{{Endpoint: "9.9.9.9:443", H3: false}}
	applyH3(pre, map[string]bool{})
	if pre[0].H3 {
		t.Errorf("empty h3Map must not set H3")
	}
	applyH3(pre, nil)
	if pre[0].H3 {
		t.Errorf("nil h3Map must not set H3")
	}
}

func selfSignedTLS(t *testing.T) *tls.Config {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "h3test.local"},
		DNSNames:     []string{"h3test.local"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: priv}},
		NextProtos:   []string{"h3"},
		MinVersion:   tls.VersionTLS13,
	}
}
