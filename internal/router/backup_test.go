package router

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
	"golang.org/x/crypto/ssh"
)

func backupTestProtection(t *testing.T) *Protection {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(key, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "identity")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0600); err != nil {
		t.Fatal(err)
	}
	protection, err := NewProtection(filepath.Join(t.TempDir(), "backups"), path, "127.0.0.1", 22)
	if err != nil {
		t.Fatal(err)
	}
	return protection
}

func backupTestArchive(t *testing.T, omit string) []byte {
	t.Helper()
	var output bytes.Buffer
	compressed := gzip.NewWriter(&output)
	archive := tar.NewWriter(compressed)
	for _, name := range []string{"etc/config/network", "etc/config/wireless", "etc/config/dhcp", "etc/config/firewall", "etc/config/gl-dns-v2", "etc/config/eth_ports_config_map", "etc/dropbear/authorized_keys"} {
		if name == omit {
			continue
		}
		data := []byte("synthetic-private-value")
		if err := archive.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestProtectionCreatesVerifiedEncryptedBackupOnce(t *testing.T) {
	protection := backupTestProtection(t)
	archive := backupTestArchive(t, "")
	calls := 0
	runner := runnerFunc(func(_ context.Context, script string) ([]byte, error) {
		if !strings.HasSuffix(script, "sysupgrade -b -\n") {
			t.Fatal("unexpected backup operation")
		}
		calls++
		return append([]byte{}, archive...), nil
	})
	for range 2 {
		if err := protection.Protect(context.Background(), runner, false); err != nil {
			t.Fatal(err)
		}
	}
	files, err := filepath.Glob(filepath.Join(protection.directory, "*.tar.gz.age"))
	if err != nil || len(files) != 1 || calls != 1 {
		t.Fatalf("backup files=%d calls=%d error=%v", len(files), calls, err)
	}
	contents, err := os.ReadFile(files[0])
	if err != nil || bytes.Contains(contents, []byte("synthetic-private-value")) || !bytes.HasPrefix(contents, []byte("age-encryption.org/v1")) {
		t.Fatal("backup is not encrypted")
	}
	reader, err := age.Decrypt(bytes.NewReader(contents), protection.identity)
	if err != nil {
		t.Fatal(err)
	}
	decrypted, err := io.ReadAll(reader)
	if err != nil || !bytes.Equal(decrypted, archive) {
		t.Fatal("backup plaintext differs from the complete router archive")
	}
	for path, want := range map[string]os.FileMode{protection.directory: 0700, files[0]: 0600} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != want {
			t.Fatalf("incorrect permissions for backup artifact: %v", err)
		}
	}
	contents[len(contents)-1] ^= 1
	if err := os.WriteFile(files[0], contents, 0600); err != nil {
		t.Fatal(err)
	}
	if err := protection.verifyBackup(files[0]); err == nil {
		t.Fatal("tampered encrypted backup was accepted")
	}
}

func TestProtectionRejectsIncompleteBackupAndDoesNotCacheFailure(t *testing.T) {
	for _, name := range []string{"missing configuration", "truncated archive", "remote failure"} {
		t.Run(name, func(t *testing.T) {
			protection := backupTestProtection(t)
			archive := backupTestArchive(t, "etc/config/firewall")
			if name == "truncated archive" {
				archive = backupTestArchive(t, "")
				archive = archive[:len(archive)-8]
			}
			runner := runnerFunc(func(context.Context, string) ([]byte, error) {
				if name == "remote failure" {
					return nil, errors.New("synthetic remote secret")
				}
				return append([]byte{}, archive...), nil
			})
			if err := protection.Protect(context.Background(), runner, false); err == nil || strings.Contains(err.Error(), "synthetic remote secret") {
				t.Fatalf("invalid backup result: %v", err)
			}
			files, _ := os.ReadDir(protection.directory)
			if protection.backedUp || len(files) != 0 {
				t.Fatal("failed backup was retained or reused")
			}
			valid := backupTestArchive(t, "")
			if err := protection.Protect(context.Background(), runnerFunc(func(context.Context, string) ([]byte, error) { return append([]byte{}, valid...), nil }), false); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestProtectionRejectsSymlinkBackupDirectory(t *testing.T) {
	protection := backupTestProtection(t)
	if err := os.Symlink(t.TempDir(), protection.directory); err != nil {
		t.Fatal(err)
	}
	if err := protection.backup(context.Background(), runnerFunc(func(context.Context, string) ([]byte, error) { t.Fatal("backup must not start"); return nil, nil })); err == nil {
		t.Fatal("symlink backup directory was accepted")
	}
}

func TestDisruptiveProtectionRequiresManagementPathBeforeBackup(t *testing.T) {
	protection := backupTestProtection(t)
	protection.backedUp = true
	calls := 0
	err := protection.Protect(context.Background(), runnerFunc(func(_ context.Context, script string) ([]byte, error) {
		calls++
		if strings.Contains(script, "sysupgrade") {
			t.Fatal("backup was attempted without a management path")
		}
		return []byte("192.168.8.1\n255.255.255.0\n"), nil
	}), true)
	if err == nil || calls != 1 {
		t.Fatalf("disruptive protection was bypassed: %v", err)
	}
}

func TestActiveEthernetLinkRejectsWirelessAndInactiveLinks(t *testing.T) {
	for _, value := range []string{"media: autoselect\nstatus: active", "media: autoselect (1000baseT <full-duplex>)\nstatus: inactive"} {
		if activeEthernetLink(value) {
			t.Fatal("unproven wired link was accepted")
		}
	}
	if !activeEthernetLink("media: autoselect (1000baseT <full-duplex>)\nstatus: active") {
		t.Fatal("active wired link was rejected")
	}
}

type streamingBackupRunner struct {
	runnerFunc
	stream func(io.Writer) error
}

func (r streamingBackupRunner) RunTo(_ context.Context, _ string, writer io.Writer) error {
	return r.stream(writer)
}

func TestProtectionStreamsBackupsAndRejectsStreamFailures(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "interrupted"}[fail], func(t *testing.T) {
			p := backupTestProtection(t)
			data := backupTestArchive(t, "")
			remote := streamingBackupRunner{runnerFunc: func(context.Context, string) ([]byte, error) {
				t.Fatal("backup was buffered instead of streamed")
				return nil, nil
			}, stream: func(w io.Writer) error {
				if _, err := w.Write(data); err != nil {
					return err
				}
				if fail {
					return errors.New("synthetic stream failure")
				}
				return nil
			}}
			err := p.Protect(context.Background(), remote, false)
			if (err != nil) != fail || p.backedUp == fail {
				t.Fatalf("stream result: %v", err)
			}
			files, _ := os.ReadDir(p.directory)
			if fail && len(files) != 0 {
				t.Fatal("interrupted stream published a backup")
			}
		})
	}
}

func TestBackupWriterRejectsOversizedArchiveBeforeWriting(t *testing.T) {
	var output bytes.Buffer
	writer := backupWriter{writer: &output, remaining: 3}
	if n, err := writer.Write([]byte("1234")); n != 0 || err == nil || output.Len() != 0 {
		t.Fatal("oversized input reached encryption")
	}
}
