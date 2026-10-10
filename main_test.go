package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// --- pickName ---------------------------------------------------------

func TestPickNameFormat(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	re := regexp.MustCompile(`^[a-z]+-[a-z]+$`)

	for i := 0; i < 200; i++ {
		name, err := pickName(rng, adjectives, nouns, "-", false, nil, map[string]bool{}, ".", false)
		if err != nil {
			t.Fatalf("pickName: %v", err)
		}
		if !re.MatchString(name) {
			t.Fatalf("name %q does not match %s", name, re)
		}
	}
}

func TestPickNameSuffixFormat(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	re := regexp.MustCompile(`^[a-z]+-[a-z]+-\d{2}$`)

	name, err := pickName(rng, adjectives, nouns, "-", true, nil, map[string]bool{}, ".", false)
	if err != nil {
		t.Fatalf("pickName: %v", err)
	}
	if !re.MatchString(name) {
		t.Fatalf("name %q does not match %s", name, re)
	}
}

func TestPickNameCustomSeparator(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	re := regexp.MustCompile(`^[a-z]+_[a-z]+$`)

	name, err := pickName(rng, adjectives, nouns, "_", false, nil, map[string]bool{}, ".", false)
	if err != nil {
		t.Fatalf("pickName: %v", err)
	}
	if !re.MatchString(name) {
		t.Fatalf("name %q does not match %s", name, re)
	}
}

func TestPickNameAvoidsHistoryAndBatch(t *testing.T) {
	adjs := []string{"a"}
	ns := []string{"b"}
	rng := rand.New(rand.NewSource(1))

	// Only one possible plain combination ("a-b"); block it via history so
	// the first phase is exhausted and we fall back to a suffixed name.
	history := map[string]bool{"a-b": true}
	batch := map[string]bool{}

	name, err := pickName(rng, adjs, ns, "-", false, history, batch, ".", false)
	if err != nil {
		t.Fatalf("pickName: %v", err)
	}
	if !regexp.MustCompile(`^a-b-\d{4}$`).MatchString(name) {
		t.Fatalf("expected suffixed fallback, got %q", name)
	}
}

func TestPickNameAvoidsFilesystemCollision(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "a-b"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	rng := rand.New(rand.NewSource(1))
	name, err := pickName(rng, []string{"a"}, []string{"b"}, "-", false, nil, map[string]bool{}, dir, true)
	if err != nil {
		t.Fatalf("pickName: %v", err)
	}
	if name == "a-b" {
		t.Fatalf("pickName returned a name colliding with an existing directory: %q", name)
	}
	if !regexp.MustCompile(`^a-b-\d{4}$`).MatchString(name) {
		t.Fatalf("expected suffixed fallback avoiding fs collision, got %q", name)
	}
}

func TestPickNameFirstPhaseExhaustionFallsBackToSuffix(t *testing.T) {
	old := maxPickAttempts
	maxPickAttempts = 3
	t.Cleanup(func() { maxPickAttempts = old })

	rng := rand.New(rand.NewSource(1))
	// The only plain combination ("a-b") is blocked, so the first phase
	// must exhaust its (shrunk) attempt budget before the suffix phase
	// kicks in and finds a free slot.
	history := map[string]bool{"a-b": true}
	dir := t.TempDir()

	name, err := pickName(rng, []string{"a"}, []string{"b"}, "-", false, history, map[string]bool{}, dir, true)
	if err != nil {
		t.Fatalf("pickName: unexpected error: %v", err)
	}
	if !regexp.MustCompile(`^a-b-\d{4}$`).MatchString(name) {
		t.Fatalf("expected suffixed fallback, got %q", name)
	}
}

func TestPickNameTrulyExhausted(t *testing.T) {
	old := maxPickAttempts
	maxPickAttempts = 50
	t.Cleanup(func() { maxPickAttempts = old })

	rng := rand.New(rand.NewSource(1))
	history := map[string]bool{}
	// Block "a-b" and every 4-digit suffixed variant that maxPickAttempts
	// tries could plausibly land on, by rejecting everything through a
	// batch populated with all 10000 suffix combinations plus the bare
	// name. This makes both phases exhaust deterministically.
	batch := map[string]bool{"a-b": true}
	for i := 0; i < 10000; i++ {
		batch[fmt.Sprintf("a-b-%04d", i)] = true
	}

	_, err := pickName(rng, []string{"a"}, []string{"b"}, "-", false, history, batch, ".", false)
	if err == nil {
		t.Fatal("expected an error when the name space is fully exhausted")
	}
}

// --- createProjectDir ---------------------------------------------------

func TestCreateProjectDirSuccess(t *testing.T) {
	base := t.TempDir()
	var stderr bytes.Buffer

	path, err := createProjectDir(base, "quiet-falcon", false, &stderr)
	if err != nil {
		t.Fatalf("createProjectDir: %v", err)
	}
	if info, statErr := os.Stat(path); statErr != nil || !info.IsDir() {
		t.Fatalf("expected %q to be a directory, stat err: %v", path, statErr)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("expected an absolute path, got %q", path)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestCreateProjectDirAlreadyExists(t *testing.T) {
	base := t.TempDir()
	if err := os.Mkdir(filepath.Join(base, "taken"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	var stderr bytes.Buffer
	_, err := createProjectDir(base, "taken", false, &stderr)
	if err == nil {
		t.Fatal("expected an error when the directory already exists")
	}
}

func TestCreateProjectDirGitInit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}

	base := t.TempDir()
	var stderr bytes.Buffer
	path, err := createProjectDir(base, "proj", true, &stderr)
	if err != nil {
		t.Fatalf("createProjectDir: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(path, ".git")); statErr != nil {
		t.Fatalf("expected .git to exist, stat err: %v", statErr)
	}
}

func TestCreateProjectDirGitNotFound(t *testing.T) {
	t.Setenv("PATH", "")

	base := t.TempDir()
	var stderr bytes.Buffer
	path, err := createProjectDir(base, "proj", true, &stderr)
	if err != nil {
		t.Fatalf("createProjectDir: %v", err)
	}
	if info, statErr := os.Stat(path); statErr != nil || !info.IsDir() {
		t.Fatalf("expected directory to still be created, stat err: %v", statErr)
	}
	if !strings.Contains(stderr.String(), "git not found") {
		t.Fatalf("expected a 'git not found' warning, got %q", stderr.String())
	}
}

// --- history --------------------------------------------------------------

func TestLoadHistoryMissingFile(t *testing.T) {
	set, err := loadHistory(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("loadHistory: %v", err)
	}
	if len(set) != 0 {
		t.Fatalf("expected an empty set, got %v", set)
	}
}

func TestLoadHistoryIgnoresBlankLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	content := "quiet-falcon\n\n  \nfond-deer\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	set, err := loadHistory(path)
	if err != nil {
		t.Fatalf("loadHistory: %v", err)
	}
	want := map[string]bool{"quiet-falcon": true, "fond-deer": true}
	if len(set) != len(want) {
		t.Fatalf("got %v, want %v", set, want)
	}
	for k := range want {
		if !set[k] {
			t.Fatalf("missing %q in %v", k, set)
		}
	}
}

func TestAppendHistoryCreatesAndAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")

	if err := appendHistory(path, []string{"quiet-falcon"}); err != nil {
		t.Fatalf("appendHistory (create): %v", err)
	}
	if err := appendHistory(path, []string{"fond-deer", "bold-otter"}); err != nil {
		t.Fatalf("appendHistory (append): %v", err)
	}

	set, err := loadHistory(path)
	if err != nil {
		t.Fatalf("loadHistory: %v", err)
	}
	for _, name := range []string{"quiet-falcon", "fond-deer", "bold-otter"} {
		if !set[name] {
			t.Fatalf("expected %q in history, got %v", name, set)
		}
	}
}

func TestDefaultHistoryPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got := defaultHistoryPath()
	want := filepath.Join(home, ".novo_history")
	if got != want {
		t.Fatalf("defaultHistoryPath() = %q, want %q", got, want)
	}
}

func TestExample(t *testing.T) {
	if got, want := example(), adjectives[0]+"-"+nouns[0]; got != want {
		t.Fatalf("example() = %q, want %q", got, want)
	}
}

// --- run (CLI) --------------------------------------------------------------

func runCLI(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	code = run(args, &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), code
}

func TestRunPrintsOneName(t *testing.T) {
	dir := t.TempDir()
	out, _, code := runCLI(t, "--print", "--no-history", "--no-fscheck", "--dir", dir)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected exactly one line, got %q", out)
	}
	if !regexp.MustCompile(`^[a-z]+-[a-z]+$`).MatchString(lines[0]) {
		t.Fatalf("unexpected name format: %q", lines[0])
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("-print must not create anything, found %v", entries)
	}
}

func TestRunCountPrintsDistinctNames(t *testing.T) {
	out, _, code := runCLI(t, "--print", "-n", "5", "--no-history", "--no-fscheck")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d (%q)", len(lines), out)
	}
	seen := map[string]bool{}
	for _, l := range lines {
		if seen[l] {
			t.Fatalf("duplicate name %q across a single run", l)
		}
		seen[l] = true
	}
}

func TestRunSuffixFlag(t *testing.T) {
	out, _, code := runCLI(t, "--print", "--suffix", "--no-history", "--no-fscheck")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !regexp.MustCompile(`^[a-z]+-[a-z]+-\d{2}$`).MatchString(strings.TrimSpace(out)) {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRunSepFlag(t *testing.T) {
	out, _, code := runCLI(t, "--print", "--sep", "_", "--no-history", "--no-fscheck")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !regexp.MustCompile(`^[a-z]+_[a-z]+$`).MatchString(strings.TrimSpace(out)) {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRunStats(t *testing.T) {
	out, _, code := runCLI(t, "--stats")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "combinations") {
		t.Fatalf("unexpected stats output: %q", out)
	}
}

func TestRunClearHistoryExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	if err := os.WriteFile(path, []byte("quiet-falcon\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, errOut, code := runCLI(t, "--clear-history", "--history-file", path)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(errOut, "history cleared") {
		t.Fatalf("unexpected stderr: %q", errOut)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected history file to be removed, stat err: %v", err)
	}
}

func TestRunClearHistoryMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist")
	_, _, code := runCLI(t, "--clear-history", "--history-file", path)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 when the history file doesn't exist", code)
	}
}

func TestRunHistoryPersistsAcrossInvocations(t *testing.T) {
	historyPath := filepath.Join(t.TempDir(), "history")

	out1, _, code := runCLI(t, "--print", "--no-fscheck", "--history-file", historyPath)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	name1 := strings.TrimSpace(out1)

	set, err := loadHistory(historyPath)
	if err != nil {
		t.Fatalf("loadHistory: %v", err)
	}
	if !set[name1] {
		t.Fatalf("expected %q to be recorded in history, got %v", name1, set)
	}

	out2, _, code := runCLI(t, "--print", "--no-fscheck", "--history-file", historyPath)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	name2 := strings.TrimSpace(out2)
	if name2 == name1 {
		t.Fatalf("expected a different name on the second run, got %q twice", name1)
	}
}

func TestRunCreatesDirectoryByDefault(t *testing.T) {
	dir := t.TempDir()
	out, _, code := runCLI(t, "--no-history", "--dir", dir)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	path := strings.TrimSpace(out)
	if !filepath.IsAbs(path) {
		t.Fatalf("expected an absolute path, got %q", path)
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		t.Fatalf("expected %q to be a directory, stat err: %v", path, err)
	}
}

func TestRunCreateAvoidsCollisionAcrossInvocations(t *testing.T) {
	dir := t.TempDir()

	out1, _, code := runCLI(t, "--no-history", "--dir", dir, "--seed", "1")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	out2, _, code := runCLI(t, "--no-history", "--dir", dir, "--seed", "1")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	path1, path2 := strings.TrimSpace(out1), strings.TrimSpace(out2)
	if path1 == path2 {
		t.Fatalf("expected the second run to avoid colliding with the first, got %q twice", path1)
	}
	for _, p := range []string{path1, path2} {
		if info, err := os.Stat(p); err != nil || !info.IsDir() {
			t.Fatalf("expected %q to be a directory, stat err: %v", p, err)
		}
	}
}

func TestRunGitInitsCreatedDirectory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}

	dir := t.TempDir()
	out, _, code := runCLI(t, "--git", "--no-history", "--dir", dir)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	path := strings.TrimSpace(out)
	if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
		t.Fatalf("expected .git to exist under %q, stat err: %v", path, err)
	}
}

func TestRunHelpExitsZero(t *testing.T) {
	_, errOut, code := runCLI(t, "--help")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(errOut, "Usage: novo") {
		t.Fatalf("expected usage text on stderr, got %q", errOut)
	}
}

func TestRunUnknownFlagExitsNonZero(t *testing.T) {
	_, _, code := runCLI(t, "--does-not-exist")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestRunRejectsNonPositiveCount(t *testing.T) {
	for _, n := range []string{"0", "-1"} {
		out, errOut, code := runCLI(t, "--print", "-n", n, "--no-history", "--no-fscheck")
		if code != 2 {
			t.Fatalf("-n %s: exit code = %d, want 2", n, code)
		}
		if out != "" {
			t.Fatalf("-n %s: expected no stdout, got %q", n, out)
		}
		if !strings.Contains(errOut, "-n must be a positive integer") {
			t.Fatalf("-n %s: unexpected stderr: %q", n, errOut)
		}
	}
}

func TestRunPickNameFailurePropagatesAsError(t *testing.T) {
	old := maxPickAttempts
	maxPickAttempts = 1
	t.Cleanup(func() { maxPickAttempts = old })

	// -n asking for more names than the (shrunk) attempt budget can ever
	// supply forces pickName's error path, which run() must surface as a
	// non-zero exit code rather than panicking or exiting 0.
	_, errOut, code := runCLI(t, "--print", "-n", "100000", "--no-history", "--no-fscheck")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (stderr: %q)", code, errOut)
	}
	if !strings.Contains(errOut, "novo:") {
		t.Fatalf("expected an error message on stderr, got %q", errOut)
	}
}

func TestRunFailureStillRecordsPrintedNames(t *testing.T) {
	old := maxPickAttempts
	maxPickAttempts = 1
	t.Cleanup(func() { maxPickAttempts = old })

	historyPath := filepath.Join(t.TempDir(), "history")
	out, _, code := runCLI(t, "--print", "-n", "100000", "--no-fscheck", "--history-file", historyPath)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	printed := strings.Fields(out)
	if len(printed) == 0 {
		t.Fatal("expected some names before the failure")
	}

	set, err := loadHistory(historyPath)
	if err != nil {
		t.Fatalf("loadHistory: %v", err)
	}
	for _, name := range printed {
		if !set[name] {
			t.Fatalf("printed name %q missing from history", name)
		}
	}
}

func TestRunPrintWithGitIsRejected(t *testing.T) {
	_, errOut, code := runCLI(t, "--print", "--git", "--no-history")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (stderr: %q)", code, errOut)
	}
}

func TestRunHistorySaveFailureIsOnlyAWarning(t *testing.T) {
	dir := t.TempDir()
	// A history file in a missing directory loads as empty but can't be written.
	historyPath := filepath.Join(dir, "missing", "history")
	out, errOut, code := runCLI(t, "--dir", dir, "--history-file", historyPath)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, errOut)
	}
	if !strings.Contains(errOut, "warning: saving history") {
		t.Fatalf("expected a history warning, got %q", errOut)
	}
	if info, err := os.Stat(strings.TrimSpace(out)); err != nil || !info.IsDir() {
		t.Fatalf("expected the created dir path on stdout, got %q", out)
	}
}

// --- shell/novo.sh ----------------------------------------------------------

func TestShellWrapperCdsIntoNewDir(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available on PATH")
	}
	bin := t.TempDir()
	if out, err := exec.Command("go", "build", "-o", filepath.Join(bin, "novo"), ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	script, err := filepath.Abs(filepath.Join("shell", "novo.sh"))
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()

	cmd := exec.Command("bash", "-c", `source "$1" && cd "$2" && novo --no-history >/dev/null && pwd -P`, "bash", script, work)
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("wrapper failed: %v", err)
	}

	got := strings.TrimSpace(string(out))
	realWork, err := filepath.EvalSymlinks(work)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(got) != realWork {
		t.Fatalf("expected the shell to end up in a new dir under %q, got %q", realWork, got)
	}
}

func TestRunShellInitPrintsWrapper(t *testing.T) {
	out, _, code := runCLI(t, "--shell-init")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "novo() {") {
		t.Fatalf("expected the shell function, got %q", out)
	}
}
