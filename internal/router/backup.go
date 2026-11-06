package router

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"filippo.io/age"
	"filippo.io/age/agessh"
	"golang.org/x/crypto/ssh"
)

type Protection struct {
	directory string
	identity  age.Identity
	recipient age.Recipient
	host      string
	port      int
	backedUp  bool
}

func NewProtection(directory, keyFile, host string, port int) (*Protection, error) {
	if directory == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return nil, errors.New("Cannot determine the backup location. Set backup_directory or GLINET_BACKUP_DIRECTORY.")
		}
		directory = filepath.Join(base, "glinet", "backups")
	}
	directory, err := filepath.Abs(directory)
	if err != nil {
		return nil, errors.New("Cannot resolve backup_directory.")
	}
	key, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, errors.New("Cannot read the SSH private key for encrypted backups.")
	}
	defer clear(key)
	identity, err := agessh.ParseIdentity(key)
	if err != nil {
		return nil, errors.New("Encrypted backups require an unencrypted Ed25519 or RSA SSH private key.")
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, errors.New("Cannot read the SSH backup recipient.")
	}
	recipient, err := agessh.ParseRecipient(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	if err != nil {
		return nil, errors.New("Cannot use the SSH public key for encrypted backups.")
	}
	return &Protection{directory: directory, identity: identity, recipient: recipient, host: host, port: port}, nil
}

func (p *Protection) Protect(ctx context.Context, runner Runner, disruptive bool) error {
	if disruptive {
		if err := verifyWiredManagement(ctx, runner, p.host, p.port); err != nil {
			return err
		}
	}
	if p.backedUp {
		return nil
	}
	if err := p.backup(ctx, runner); err != nil {
		return fmt.Errorf("No router configuration was changed because the encrypted recovery backup failed: %w", err)
	}
	p.backedUp = true
	return nil
}

func (p *Protection) backup(ctx context.Context, runner Runner) error {
	if err := os.MkdirAll(p.directory, 0700); err != nil {
		return errors.New("Cannot create backup_directory.")
	}
	info, err := os.Lstat(p.directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("backup_directory must be a real directory, not a symbolic link.")
	}
	if err := os.Chmod(p.directory, 0700); err != nil {
		return errors.New("Cannot restrict backup_directory permissions.")
	}
	file, err := os.CreateTemp(p.directory, ".backup-*.age")
	if err != nil {
		return errors.New("Cannot create the encrypted backup file.")
	}
	temporary := file.Name()
	defer func() {
		_ = file.Close()
		_ = os.Remove(temporary)
	}()
	encrypted, err := age.Encrypt(file, p.recipient)
	if err != nil {
		return errors.New("Cannot initialize backup encryption.")
	}
	limited := &backupWriter{writer: encrypted, remaining: 32 << 20}
	if streaming, ok := runner.(interface {
		RunTo(context.Context, string, io.Writer) error
	}); ok {
		err = streaming.RunTo(ctx, preflight+"sysupgrade -b -\n", limited)
	} else {
		var data []byte
		data, err = runner.Run(ctx, preflight+"sysupgrade -b -\n")
		if err == nil {
			_, err = limited.Write(data)
		}
		clear(data)
	}
	if err != nil {
		return errors.New("Cannot read the complete router recovery archive.")
	}
	if err := encrypted.Close(); err != nil {
		return errors.New("Cannot finish backup encryption.")
	}
	if err := file.Sync(); err != nil {
		return errors.New("Cannot persist the encrypted backup.")
	}
	if err := file.Close(); err != nil {
		return errors.New("Cannot close the encrypted backup.")
	}
	if err := p.verifyBackup(temporary); err != nil {
		return err
	}
	token := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(temporary), ".backup-"), ".age")
	destination := filepath.Join(p.directory, "backup-"+time.Now().UTC().Format("20060102T150405.000000000Z")+"-"+token+".tar.gz.age")
	if err := os.Link(temporary, destination); err != nil {
		return errors.New("Cannot publish the verified encrypted backup.")
	}
	directory, err := os.Open(p.directory)
	if err != nil {
		return errors.New("Cannot open the backup directory for synchronization.")
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return errors.New("Cannot persist the backup directory.")
	}
	return nil
}

type backupWriter struct {
	writer    io.Writer
	remaining int64
}

func (w *backupWriter) Write(data []byte) (int, error) {
	if int64(len(data)) > w.remaining {
		return 0, errors.New("The recovery archive exceeds the supported size.")
	}
	n, err := w.writer.Write(data)
	w.remaining -= int64(n)
	return n, err
}

func (p *Protection) verifyBackup(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return errors.New("Cannot reopen the encrypted backup for verification.")
	}
	defer file.Close()
	plaintext, err := age.Decrypt(file, p.identity)
	if err != nil {
		return errors.New("Cannot decrypt the recovery backup with the SSH identity.")
	}
	limited := &io.LimitedReader{R: plaintext, N: 32 << 20}
	compressed, err := gzip.NewReader(limited)
	if err != nil {
		return errors.New("The recovery backup is not a gzip archive.")
	}
	defer compressed.Close()
	decompressed := &io.LimitedReader{R: compressed, N: 64 << 20}
	archive := tar.NewReader(decompressed)
	required := map[string]bool{
		"etc/config/network": false, "etc/config/wireless": false,
		"etc/config/dhcp": false, "etc/config/firewall": false,
		"etc/config/gl-dns-v2": false, "etc/config/eth_ports_config_map": false,
		"etc/dropbear/authorized_keys": false,
	}
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return errors.New("The recovery backup contains an incomplete tar archive.")
		}
		name := strings.TrimPrefix(header.Name, "./")
		if _, ok := required[name]; ok {
			if header.Typeflag != tar.TypeReg || header.Size == 0 {
				return errors.New("A required recovery file is empty or is not a regular file.")
			}
			required[name] = true
		}
	}
	if _, err := io.Copy(io.Discard, decompressed); err != nil || decompressed.N == 0 {
		return errors.New("The recovery backup failed gzip integrity verification.")
	}
	if _, err := io.Copy(io.Discard, limited); err != nil || limited.N == 0 {
		return errors.New("The recovery backup failed age integrity verification.")
	}
	for _, found := range required {
		if !found {
			return errors.New("The recovery backup is missing a required configuration or SSH authorization file.")
		}
	}
	return nil
}

var _ io.Writer = (*backupWriter)(nil)
