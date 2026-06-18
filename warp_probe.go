package main

import (
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash"
	"net"
	"time"

	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/chacha20poly1305"
)

// Native WireGuard (Noise_IKpsk2_25519_ChaChaPoly_BLAKE2s) handshake-initiation
// prober. Unlike a plain TCP dial — meaningless against WARP's UDP-only
// WireGuard ports — this sends a real, cryptographically valid handshake using
// the uploaded .conf's registered credentials and waits for the responder's
// handshake response. It validates the correct protocol and is far faster than
// spinning up an xray process per endpoint (no process, no SOCKS hop).
//
// WARP carries the per-account client id in the WireGuard header's three
// "reserved" bytes (the S1/S2/S3 / Reserved triple), so those must be set
// before MAC1 is computed — MAC1 covers them.

var (
	wgConstruction = []byte("Noise_IKpsk2_25519_ChaChaPoly_BLAKE2s")
	wgIdentifier   = []byte("WireGuard v1 zx2c4 Jason@zx2c4.com")
	wgLabelMAC1    = []byte("mac1----")
)

// blake2sHash returns BLAKE2s-256 over the concatenation of the inputs.
func blake2sHash(parts ...[]byte) [blake2s.Size]byte {
	h, _ := blake2s.New256(nil)
	for _, p := range parts {
		h.Write(p)
	}
	var out [blake2s.Size]byte
	h.Sum(out[:0])
	return out
}

// wgHMAC is HMAC-BLAKE2s-256, the primitive underlying WireGuard's KDF.
func wgHMAC(key, data []byte) [blake2s.Size]byte {
	mac := hmac.New(func() hash.Hash { h, _ := blake2s.New256(nil); return h }, key)
	mac.Write(data)
	var out [blake2s.Size]byte
	mac.Sum(out[:0])
	return out
}

// kdf1/kdf2 implement WireGuard's HKDF-style key derivation.
func kdf1(key, input []byte) [blake2s.Size]byte {
	prk := wgHMAC(key, input)
	return wgHMAC(prk[:], []byte{0x1})
}

func kdf2(key, input []byte) (t0, t1 [blake2s.Size]byte) {
	prk := wgHMAC(key, input)
	t0 = wgHMAC(prk[:], []byte{0x1})
	t1 = wgHMAC(prk[:], append(append([]byte{}, t0[:]...), 0x2))
	return t0, t1
}

// wgMAC is keyed BLAKE2s with a 128-bit digest (used for MAC1).
func wgMAC(key, data []byte) [16]byte {
	h, _ := blake2s.New128(key)
	h.Write(data)
	var out [16]byte
	h.Sum(out[:0])
	return out
}

// tai64n encodes a timestamp the way WireGuard expects (TAI64N, 12 bytes).
func tai64n(t time.Time) [12]byte {
	var ts [12]byte
	binary.BigEndian.PutUint64(ts[0:8], uint64(t.Unix())+0x400000000000000a)
	binary.BigEndian.PutUint32(ts[8:12], uint32(t.Nanosecond()))
	return ts
}

// warpProber holds the handshake material that is constant across endpoints for
// a given WARP config. Building it performs the expensive X25519 static-static
// DH and key parsing exactly once, so a scan of thousands of endpoints reuses
// that work instead of repeating it per probe.
type warpProber struct {
	curve      ecdh.Curve
	staticPriv *ecdh.PrivateKey
	peerPub    *ecdh.PublicKey
	peerBytes  []byte
	staticPub  []byte
	dh2        []byte             // DH(static, peer-static) — endpoint-independent
	mac1Key    [blake2s.Size]byte // HASH(LABEL_MAC1 || peer-static)
	reserved   [3]byte            // WARP client-id carried in the reserved triple
}

// newWarpProber precomputes the per-config handshake constants.
func newWarpProber(cfg *WarpConfig) (*warpProber, error) {
	privBytes, err := base64.StdEncoding.DecodeString(cfg.PrivateKey)
	if err != nil || len(privBytes) != 32 {
		return nil, fmt.Errorf("invalid private key")
	}
	peerBytes, err := base64.StdEncoding.DecodeString(cfg.PublicKey)
	if err != nil || len(peerBytes) != 32 {
		return nil, fmt.Errorf("invalid peer public key")
	}

	curve := ecdh.X25519()
	staticPriv, err := curve.NewPrivateKey(privBytes)
	if err != nil {
		return nil, fmt.Errorf("static private key: %w", err)
	}
	peerPub, err := curve.NewPublicKey(peerBytes)
	if err != nil {
		return nil, fmt.Errorf("peer public key: %w", err)
	}

	// DH(static, peer-static) does not depend on the endpoint or the per-probe
	// ephemeral key, so compute it once here rather than once per endpoint.
	dh2, err := staticPriv.ECDH(peerPub)
	if err != nil {
		return nil, err
	}

	p := &warpProber{
		curve:      curve,
		staticPriv: staticPriv,
		peerPub:    peerPub,
		peerBytes:  peerBytes,
		staticPub:  staticPriv.PublicKey().Bytes(),
		dh2:        dh2,
		mac1Key:    blake2sHash(wgLabelMAC1, peerBytes),
	}
	if len(cfg.Reserved) >= 3 {
		p.reserved[0] = byte(cfg.Reserved[0])
		p.reserved[1] = byte(cfg.Reserved[1])
		p.reserved[2] = byte(cfg.Reserved[2])
	}
	return p, nil
}

// Probe performs a single WireGuard handshake against endpoint and returns the
// round-trip time to the handshake response. A non-nil error means the endpoint
// did not complete a handshake within timeout (unreachable, wrong protocol, or
// wrong credentials). The initiation is retransmitted periodically within the
// timeout window so a single dropped UDP datagram does not condemn an otherwise
// reachable endpoint.
func (p *warpProber) Probe(endpoint string, timeout time.Duration) (time.Duration, error) {
	ephPriv, err := p.curve.GenerateKey(rand.Reader)
	if err != nil {
		return 0, err
	}
	ephPub := ephPriv.PublicKey().Bytes()

	// Noise IKpsk2 initiator hash/chain init.
	ck := blake2sHash(wgConstruction)
	h := blake2sHash(ck[:], wgIdentifier)
	h = blake2sHash(h[:], p.peerBytes)

	// Mix in the ephemeral public key.
	ck = kdf1(ck[:], ephPub)
	h = blake2sHash(h[:], ephPub)

	// Encrypt our static public key under DH(eph, peer-static).
	dh1, err := ephPriv.ECDH(p.peerPub)
	if err != nil {
		return 0, err
	}
	var key [blake2s.Size]byte
	ck, key = kdf2(ck[:], dh1)
	aeadStatic, err := chacha20poly1305.New(key[:])
	if err != nil {
		return 0, err
	}
	var zeroNonce [12]byte
	encStatic := aeadStatic.Seal(nil, zeroNonce[:], p.staticPub, h[:]) // 48 bytes
	h = blake2sHash(h[:], encStatic)

	// Encrypt the timestamp under the precomputed DH(static, peer-static).
	ck, key = kdf2(ck[:], p.dh2)
	aeadTime, err := chacha20poly1305.New(key[:])
	if err != nil {
		return 0, err
	}
	ts := tai64n(time.Now())
	encTime := aeadTime.Seal(nil, zeroNonce[:], ts[:], h[:]) // 28 bytes
	_ = ck                                                   // chain key not needed past this point for a probe

	// Assemble the 148-byte handshake initiation.
	msg := make([]byte, 148)
	msg[0] = 1 // message type 1 (handshake initiation); bytes 1..3 are reserved
	msg[1] = p.reserved[0]
	msg[2] = p.reserved[1]
	msg[3] = p.reserved[2]
	var idxBuf [4]byte
	if _, err := rand.Read(idxBuf[:]); err != nil {
		return 0, err
	}
	senderIndex := binary.LittleEndian.Uint32(idxBuf[:])
	binary.LittleEndian.PutUint32(msg[4:8], senderIndex)
	copy(msg[8:40], ephPub)
	copy(msg[40:88], encStatic)
	copy(msg[88:116], encTime)

	// MAC1 = keyed-BLAKE2s(HASH(LABEL_MAC1 || peer-static), msg[0:116]).
	mac1 := wgMAC(p.mac1Key[:], msg[0:116])
	copy(msg[116:132], mac1[:])
	// MAC2 stays zero (no cookie challenge outstanding).

	conn, err := net.DialTimeout("udp", endpoint, timeout)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	deadline := time.Now().Add(timeout)
	// Retransmit interval: a fraction of the budget, clamped so short timeouts
	// still get at least one resend and long ones don't flood the path.
	retransmit := timeout / 3
	if retransmit > 700*time.Millisecond {
		retransmit = 700 * time.Millisecond
	}
	if retransmit < 100*time.Millisecond {
		retransmit = 100 * time.Millisecond
	}

	sendTime := time.Now()
	if _, err := conn.Write(msg); err != nil {
		return 0, err
	}

	buf := make([]byte, 256)
	for {
		now := time.Now()
		if !now.Before(deadline) {
			return 0, fmt.Errorf("no handshake response within %v", timeout)
		}
		readUntil := now.Add(retransmit)
		if readUntil.After(deadline) {
			readUntil = deadline
		}
		conn.SetReadDeadline(readUntil)

		n, err := conn.Read(buf)
		if err != nil {
			// A per-read timeout with budget left means the datagram was likely
			// lost — retransmit the identical initiation and keep waiting.
			if ne, ok := err.(net.Error); ok && ne.Timeout() && time.Now().Before(deadline) {
				sendTime = time.Now()
				if _, werr := conn.Write(msg); werr != nil {
					return 0, werr
				}
				continue
			}
			return 0, err
		}
		// Handshake response (type 2, 92 bytes): receiver index echoes ours.
		if n >= 92 && buf[0] == 2 && binary.LittleEndian.Uint32(buf[8:12]) == senderIndex {
			return time.Since(sendTime), nil
		}
		// Cookie reply (type 3): still proves a live WireGuard responder.
		if n >= 64 && buf[0] == 3 && binary.LittleEndian.Uint32(buf[4:8]) == senderIndex {
			return time.Since(sendTime), nil
		}
		// Anything else: keep reading until the deadline fires.
	}
}
