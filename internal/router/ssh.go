package router

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type Runner interface {
	Run(context.Context, string) ([]byte, error)
}

type ExitError struct{ Code int }

func (e *ExitError) Error() string { return "The router rejected the operation." }

type SSH struct {
	address string
	config  *ssh.ClientConfig
}

func NewSSH(host string, port int, keyFile, knownHostsFile string) (*SSH, error) {
	if host == "" || strings.ContainsAny(host, "\x00\r\n") || port < 1 || port > 65535 {
		return nil, errors.New("Set a router hostname and an SSH port between 1 and 65535.")
	}
	key, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, errors.New("Cannot read private_key_file. Provide a readable OpenSSH private key file.")
	}
	signer, err := ssh.ParsePrivateKey(key)
	clear(key)
	if err != nil {
		return nil, errors.New("Cannot parse private_key_file. This version requires an unencrypted OpenSSH private key.")
	}
	callback, err := knownhosts.New(knownHostsFile)
	if err != nil {
		return nil, errors.New("Cannot read known_hosts_file. Verify the router host key and provide a valid known_hosts file.")
	}
	return &SSH{
		address: net.JoinHostPort(host, strconv.Itoa(port)),
		config: &ssh.ClientConfig{
			User: "root", Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
			HostKeyCallback: callback, Timeout: 10 * time.Second,
		},
	}, nil
}

type boundedOutput struct {
	data     []byte
	overflow bool
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	remaining := 4096 - len(b.data)
	if len(p) > remaining {
		b.overflow = true
		b.data = append(b.data, p[:remaining]...)
	} else {
		b.data = append(b.data, p...)
	}
	return len(p), nil
}

func (s *SSH) Run(ctx context.Context, script string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", s.address)
	if err != nil {
		return nil, errors.New("Cannot reach the router over SSH. Check host, port, and network access.")
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	channel, requests, messages, err := ssh.NewClientConn(conn, s.address, s.config)
	if err != nil {
		return nil, errors.New("SSH authentication or host-key verification failed. Check private_key_file and known_hosts_file; host-key checking cannot be disabled.")
	}
	client := ssh.NewClient(channel, requests, messages)
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return nil, errors.New("Cannot start a router SSH session.")
	}
	defer session.Close()
	output := &boundedOutput{}
	session.Stdin = strings.NewReader(script)
	session.Stdout = output
	session.Stderr = io.Discard
	if err = session.Run("sh -s"); err != nil {
		var exit *ssh.ExitError
		if errors.As(err, &exit) {
			return nil, &ExitError{Code: exit.ExitStatus()}
		}
		return nil, errors.New("The router SSH operation did not complete. Check connectivity before retrying.")
	}
	if output.overflow {
		return nil, errors.New("The router returned more data than expected.")
	}
	return output.data, nil
}
