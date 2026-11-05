package router

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func sshFixture(t *testing.T, output string, exitCode uint32, hang bool) (*SSH, *atomic.Int32) {
	t.Helper()
	_, serverKey, _ := ed25519.GenerateKey(rand.Reader)
	serverSigner, _ := ssh.NewSignerFromKey(serverKey)
	_, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	clientSigner, _ := ssh.NewSignerFromKey(privateKey)
	config := &ssh.ServerConfig{PublicKeyCallback: func(meta ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if meta.User() != "root" || !bytes.Equal(key.Marshal(), clientSigner.PublicKey().Marshal()) {
			return nil, errors.New("fixture authentication rejected")
		}
		return nil, nil
	}}
	config.AddHostKey(serverSigner)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); _ = listener.Close() })
	calls := &atomic.Int32{}
	go func() {
		for {
			network, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer network.Close()
				stop := context.AfterFunc(ctx, func() { _ = network.Close() })
				defer stop()
				connection, channels, requests, err := ssh.NewServerConn(network, config)
				if err != nil {
					return
				}
				defer connection.Close()
				go ssh.DiscardRequests(requests)
				closed := make(chan struct{})
				go func() { _ = connection.Wait(); close(closed) }()
				for pending := range channels {
					if pending.ChannelType() != "session" {
						_ = pending.Reject(ssh.UnknownChannelType, "sessions only")
						continue
					}
					channel, requests, err := pending.Accept()
					if err != nil {
						return
					}
					go func() {
						defer channel.Close()
						for request := range requests {
							var payload struct{ Command string }
							if request.Type != "exec" || ssh.Unmarshal(request.Payload, &payload) != nil || payload.Command != "sh -s" {
								_ = request.Reply(false, nil)
								continue
							}
							_ = request.Reply(true, nil)
							_, _ = io.ReadAll(channel)
							calls.Add(1)
							if hang {
								<-closed
								return
							}
							_, _ = channel.Stderr().Write([]byte("fixture-secret-in-stderr"))
							_, _ = channel.Write([]byte(output))
							_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{exitCode}))
							return
						}
					}()
				}
			}()
		}
	}()
	root := t.TempDir()
	keyFile := filepath.Join(root, "identity")
	block, err := ssh.MarshalPrivateKey(privateKey, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	knownFile := filepath.Join(root, "known_hosts")
	line := knownhosts.Line([]string{knownhosts.Normalize(listener.Addr().String())}, serverSigner.PublicKey())
	if err := os.WriteFile(knownFile, []byte(line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().(*net.TCPAddr)
	transport, err := NewSSH("127.0.0.1", address.Port, keyFile, knownFile)
	if err != nil {
		t.Fatal(err)
	}
	return transport, calls
}

func TestSSHUsesVerifiedKeyAndRedactsRemoteErrors(t *testing.T) {
	transport, calls := sshFixture(t, "ok", 0, false)
	output, err := transport.Run(context.Background(), "printf test\n")
	if err != nil || string(output) != "ok" || calls.Load() != 1 {
		t.Fatalf("Authenticated operation failed: %q, %v", output, err)
	}
	transport, _ = sshFixture(t, "fixture-secret-in-stdout", 65, false)
	output, err = transport.Run(context.Background(), "false\n")
	var exit *ExitError
	if output != nil || !errors.As(err, &exit) || exit.Code != 65 || strings.Contains(err.Error(), "fixture-secret") {
		t.Fatalf("Unsafe remote error: %q, %v", output, err)
	}
}

func TestSSHRejectsUntrustedHostAndCancelsBlockedOperation(t *testing.T) {
	transport, calls := sshFixture(t, "should-not-run", 0, false)
	_, wrongKey, _ := ed25519.GenerateKey(rand.Reader)
	wrongSigner, _ := ssh.NewSignerFromKey(wrongKey)
	wrongFile := filepath.Join(t.TempDir(), "known_hosts")
	line := knownhosts.Line([]string{knownhosts.Normalize(transport.address)}, wrongSigner.PublicKey())
	if err := os.WriteFile(wrongFile, []byte(line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	transport.config.HostKeyCallback, _ = knownhosts.New(wrongFile)
	if _, err := transport.Run(context.Background(), "true\n"); err == nil || calls.Load() != 0 {
		t.Fatal("Untrusted router host key was accepted")
	}
	transport, _ = sshFixture(t, "", 0, true)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := transport.Run(ctx, "hang\n"); err == nil || time.Since(start) > 2*time.Second {
		t.Fatal("Canceled SSH operation did not stop promptly")
	}
}

func TestSSHBoundsRemoteOutput(t *testing.T) {
	transport, _ := sshFixture(t, strings.Repeat("x", 262145), 0, false)
	if _, err := transport.Run(context.Background(), "true\n"); err == nil {
		t.Fatal("Oversized output was accepted")
	}
}
