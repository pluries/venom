package skillcheck

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// releaseServer serves <base>/releases/download/<tag>/<asset> from a folder and records every path asked.
type releaseServer struct {
	*httptest.Server
	mu    sync.Mutex
	paths []string
}

func newReleaseServer(t *testing.T, dir string) *releaseServer {
	t.Helper()
	rs := &releaseServer{}
	prefix := "/releases/download/" + releaseTag + "/"
	rs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rs.mu.Lock()
		rs.paths = append(rs.paths, r.URL.Path)
		rs.mu.Unlock()
		if !strings.HasPrefix(r.URL.Path, prefix) {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dir, path.Base(r.URL.Path)))
	}))
	t.Cleanup(rs.Close)
	return rs
}

func (rs *releaseServer) requested() []string {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return append([]string(nil), rs.paths...)
}

// sandbox holds every folder an install may touch, so a test can prove nothing else was written.
type sandbox struct {
	cwd, home, tmp, dest string
}

func newSandbox(t *testing.T) sandbox {
	t.Helper()
	root := t.TempDir()
	sb := sandbox{cwd: filepath.Join(root, "cwd"), home: filepath.Join(root, "home"),
		tmp: filepath.Join(root, "tmp"), dest: filepath.Join(root, "dest")}
	for _, d := range []string{sb.cwd, sb.home, sb.tmp, sb.dest} {
		require.NoError(t, os.Mkdir(d, 0o700))
	}
	return sb
}

// listing returns the sorted names under dir, relative to it; folders count.
func listing(t *testing.T, dir string) []string {
	t.Helper()
	var names []string
	err := filepath.WalkDir(dir, func(p string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p != dir {
			rel, relErr := filepath.Rel(dir, p)
			require.NoError(t, relErr)
			names = append(names, filepath.ToSlash(rel))
		}
		return nil
	})
	require.NoError(t, err)
	sort.Strings(names)
	return names
}

// requireNothingOutsideDest checks that the install left no file in the working folder, the home and the temp folder.
func (sb sandbox) requireNothingOutsideDest(t *testing.T) {
	t.Helper()
	require.Empty(t, listing(t, sb.cwd), "files written into the working folder")
	require.Empty(t, listing(t, sb.home), "files written into HOME")
	require.Empty(t, listing(t, sb.tmp), "files left in the temporary folder")
}

// installRun describes one run of install-venom.sh.
type installRun struct {
	sb       sandbox
	pinned   keyPair
	args     []string // after --version, --dest and --release-env
	unameS   string
	unameM   string
	extraEnv []string
	pathHead string // folder put before the PATH
}

// installEnv is the environment of an install run: a sandboxed HOME and temp folder, a fixed system, the local timestamp
// root, no proxy and a PATH without any venom.
func installEnv(t *testing.T, k *fixtureKeys, r installRun) []string {
	t.Helper()
	if r.unameS == "" {
		r.unameS, r.unameM = "Linux", "x86_64"
	}
	env := []string{"HOME=" + slash(r.sb.home), "TMPDIR=" + slash(r.sb.tmp), "TMP=" + slash(r.sb.tmp), "TEMP=" + slash(r.sb.tmp),
		"VENOM_UNAME_S=" + r.unameS, "VENOM_UNAME_M=" + r.unameM, "VENOM_CA_FILE=" + k.caPath,
		"NO_PROXY=127.0.0.1,localhost", "no_proxy=127.0.0.1,localhost", "HTTP_PROXY=", "http_proxy=", "HTTPS_PROXY=", "https_proxy="}
	path := noVenomPath(t)
	if r.pathHead != "" {
		path = r.pathHead + string(os.PathListSeparator) + path
	}
	env = append(env, "PATH="+path)
	env = append(env, r.extraEnv...)
	return mergeEnv(env...)
}

func execInstall(t *testing.T, k *fixtureKeys, r installRun) cmdResult {
	t.Helper()
	script := skillFile(t, "scripts/install-venom.sh")
	env := installEnv(t, k, r)
	args := []string{slash(script), "--version", releaseTag, "--dest", slash(r.sb.dest),
		"--release-env", slash(releaseEnvFor(t, r.pinned))}
	return bashRunWith(t, bareBashPath(t), env, r.sb.cwd, append(args, r.args...)...)
}

// requireExecutable checks the executable bit the way the scripts see it, inside bash.
func requireExecutable(t *testing.T, p string) {
	t.Helper()
	res := bashRun(t, mergeEnv(), "", "-c", `[ -x "$1" ]`, "_", slash(p))
	require.Equal(t, 0, res.code, "%s is not executable", p)
}

// installCase is one row of the table of install paths: a way to fetch, a defect in the release, a verdict.
type installCase struct {
	name     string
	viaURL   bool
	mutate   mutation
	wantCode int
	wantName string
}

func runInstallCase(t *testing.T, c installCase) {
	t.Helper()
	requireInstallTools(t)
	k := fixtureKeysFor(t)
	rel := newRelease(t, k, k.one)
	applyMutation(t, c.mutate, rel)
	sb := newSandbox(t)
	run := installRun{sb: sb, pinned: k.one}
	var srv *releaseServer
	if c.viaURL {
		srv = newReleaseServer(t, rel)
		run.args = []string{"--base-url", srv.URL}
	} else {
		run.args = []string{"--from-dir", slash(rel)}
	}
	res := execInstall(t, k, run)
	assertInstallVerdict(t, res, c, sb, srv)
}

func assertInstallVerdict(t *testing.T, res cmdResult, c installCase, sb sandbox, srv *releaseServer) {
	t.Helper()
	require.Equal(t, c.wantCode, res.code, "stdout:\n%s\nstderr:\n%s", res.stdout, res.stderr)
	sb.requireNothingOutsideDest(t)
	if c.wantCode != 0 {
		require.Contains(t, res.stderr, "FAIL "+c.wantName+":")
		require.Empty(t, listing(t, sb.dest), "nothing may reach the destination when a check fails")
		return
	}
	require.Equal(t, []string{"venom"}, listing(t, sb.dest))
	requireExecutable(t, filepath.Join(sb.dest, "venom"))
	require.Contains(t, res.stdout, "Version venom: "+releaseTag, "venom version is printed")
	if srv != nil {
		requireAllFilesAsked(t, srv.requested())
	}
}

func requireAllFilesAsked(t *testing.T, asked []string) {
	t.Helper()
	for _, name := range []string{releaseAsset, sumsName, p7sName, tsrName} {
		require.Contains(t, asked, "/releases/download/"+releaseTag+"/"+name)
	}
}

// US1-AS1 and US1-AS9, T011: the install from a download and from saved files ends with the executable in
// the destination and `venom version` printed; both paths run the same checks, so a changed byte stops both.
func TestInstall_US1AS1_AS9_DownloadAndFromDirRunTheSameChecks(t *testing.T) {
	cases := []installCase{
		{name: "US1-AS1 download", viaURL: true},
		{name: "US1-AS9 from-dir", viaURL: false},
		{name: "download, changed executable", viaURL: true, mutate: flipByte(releaseAsset, -1), wantCode: 10, wantName: "checksum"},
		{name: "from-dir, changed executable", viaURL: false, mutate: flipByte(releaseAsset, -1), wantCode: 10, wantName: "checksum"},
		{name: "download, changed signature", viaURL: true, mutate: flipByte(p7sName, -1), wantCode: 11, wantName: "signature"},
		{name: "from-dir, changed signature", viaURL: false, mutate: flipByte(p7sName, -1), wantCode: 11, wantName: "signature"},
		{name: "from-dir, changed timestamp token", viaURL: false, mutate: flipByte(tsrName, -1), wantCode: 13, wantName: "timestamp"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { runInstallCase(t, c) })
	}
}

// US1-AS7, T011: a release signed by another certificate stops the install with exit 12, whatever the path.
func TestInstall_US1AS7_OtherSignerInstallsNothing(t *testing.T) {
	requireInstallTools(t)
	k := fixtureKeysFor(t)
	sb := newSandbox(t)
	rel := newRelease(t, k, k.two)
	res := execInstall(t, k, installRun{sb: sb, pinned: k.one, args: []string{"--from-dir", slash(rel)}})
	require.Equal(t, 12, res.code, "stdout:\n%s\nstderr:\n%s", res.stdout, res.stderr)
	require.Contains(t, res.stderr, "FAIL signer:")
	require.Empty(t, listing(t, sb.dest))
	sb.requireNothingOutsideDest(t)
}
