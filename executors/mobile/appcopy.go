package mobile

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/ovh/venom"
)

// remoteAppName is the only file name venom uses on the SSH host: the suite never chooses a name or a path there.
const (
	remoteAppName = "app.apk"
	remoteZipName = "app.zip" // iOS: a .zip file, or a .app folder zipped by venom
)

// maxRemoteStderr bounds the text of a failed remote command that goes into an error.
const maxRemoteStderr = 200

// newRemoteDir is the venom-named folder for one session: 16 random hex characters from crypto/rand, so the name is
// neither guessable nor taken from the suite.
func newRemoteDir() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("cannot name the folder for the app on the SSH host: %w", err)
	}
	return "/tmp/venom-mobile-" + hex.EncodeToString(b[:]), nil
}

// runRemote runs one command on the SSH host over an exec channel of the tunnel's connection, with stdin as its input.
// A failure carries the beginning of what the command wrote on stderr.
func runRemote(ctx context.Context, client *ssh.Client, cmd string, stdin io.Reader) error {
	sess, err := client.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	stop := context.AfterFunc(ctx, func() { _ = sess.Close() })
	defer stop()
	var stderr bytes.Buffer
	sess.Stdin = stdin
	sess.Stderr = &stderr
	if err := sess.Run(cmd); err != nil {
		text := strings.TrimSpace(stderr.String())
		if len(text) > maxRemoteStderr {
			text = text[:maxRemoteStderr]
		}
		if text != "" {
			return fmt.Errorf("%w: %s", err, text)
		}
		return err
	}
	return nil
}

// stageApp sends the app file to the SSH host when the suite names one, so the server installs the copy, and points
// cfg at it. The folder is removed by a closer registered before anything is created in it: it runs on a hash mismatch,
// a failed session and TearDown alike, before the tunnel closes. The hash is computed over the bytes sent and checked
// after the stream, before any request to the server (FR-04).
func (t *tunnel) stageApp(ctx context.Context, cfg *setupConfig, opened *closers) error {
	if cfg.appPath == "" {
		return nil
	}
	local := cfg.appPath
	if cfg.appBundle {
		// a .app folder travels as a zip made here; the temporary file goes when staging ends, on failure too
		zipped, cleanup, err := zipBundle(cfg.appPath)
		if err != nil {
			return err
		}
		defer cleanup()
		local = zipped
	}
	dir, err := newRemoteDir()
	if err != nil {
		return err
	}
	*opened = append(*opened, func() error { return t.removeRemote(ctx, dir) })
	if err := runRemote(ctx, t.client, "mkdir -m 700 "+dir, nil); err != nil {
		return fmt.Errorf("cannot create the folder for the app on the SSH host: %w", err)
	}
	remote := dir + "/" + remoteNameOf(cfg.platform)
	sum, err := t.sendFile(ctx, local, remote)
	if err != nil {
		return fmt.Errorf("cannot copy mobile.app %q to the SSH host: %w", cfg.app, err)
	}
	if err := cfg.recordSent(sum); err != nil {
		return err
	}
	venom.Info(ctx, "mobile app copied to the SSH host (sha256 %s)", sum)
	cfg.appPath = remote
	return nil
}

// remoteNameOf is the file name on the SSH host: the iOS app always travels as a zip.
func remoteNameOf(platform string) string {
	if platform == platformIOS {
		return remoteZipName
	}
	return remoteAppName
}

// recordSent checks the hash of the bytes sent against mobile.appSHA256 and keeps it for the report. A .app folder was
// zipped here, so there is no hash to check or to report.
func (c *setupConfig) recordSent(sum string) error {
	if c.appBundle {
		return nil
	}
	if err := checkAppHash(c.app, sum, c.appSHA256); err != nil {
		return err
	}
	c.appSHA256 = sum
	return nil
}

// sendFile streams the local file to remote with `cat >` and returns the SHA-256 of the bytes that went out.
func (t *tunnel) sendFile(ctx context.Context, local, remote string) (string, error) {
	f, err := os.Open(local)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if err := runRemote(ctx, t.client, "cat > "+remote, io.TeeReader(f, h)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// removeRemote deletes the folder venom made on the SSH host. A failure is reported, never fatal to the test result:
// it comes back through the close error and a warning.
func (t *tunnel) removeRemote(ctx context.Context, dir string) error {
	// The step's cancellation must not skip the cleanup, and a stalled host must not hold it forever.
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), closeTimeout)
	defer cancel()
	if err := runRemote(rmCtx, t.client, "rm -rf "+dir, nil); err != nil {
		warnf(ctx, "mobile: cannot remove %s on the SSH host: %v", dir, err)
		return fmt.Errorf("cannot remove %s on the SSH host: %w", dir, err)
	}
	return nil
}
