// Command novo creates a new project directory with a mnemonic,
// docker-style name (e.g. "quiet-falcon") and prints its absolute path,
// so you never have to think up a name or clear it with anyone. With
// -print it only prints names. shell/novo.sh wraps it to also cd there.
package main

import (
	"bufio"
	_ "embed"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// shellInit is the shell function that cds into the created directory,
// printed by -shell-init for `eval "$(novo --shell-init)"` in a profile.
//
//go:embed shell/novo.sh
var shellInit string

// maxPickAttempts bounds how many random candidates pickName tries per
// phase before giving up (plain names, then suffixed names). It's a var
// rather than a const so tests can shrink it to force the exhaustion path
// deterministically, without needing a word list with tens of thousands
// of entries.
var maxPickAttempts = 500

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run implements the CLI. It never calls os.Exit itself, so tests can
// invoke it directly with captured output and assert on both the return
// code and what was written.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("novo", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		count       = fs.Int("n", 1, "how many names (directories) to generate")
		sep         = fs.String("sep", "-", "separator between the words")
		suffix      = fs.Bool("suffix", false, "append a random 2-digit number for extra entropy")
		noHistory   = fs.Bool("no-history", false, "don't check or record names in the history file")
		noFsCheck   = fs.Bool("no-fscheck", false, "don't avoid names already taken by a file/dir in -dir")
		clearHist   = fs.Bool("clear-history", false, "wipe the history file and exit")
		dir         = fs.String("dir", ".", "directory to check for existing files/dirs with the generated name")
		historyPath = fs.String("history-file", defaultHistoryPath(), "path to the history file")
		stats       = fs.Bool("stats", false, "print the size of the name space and exit")
		seed        = fs.Int64("seed", 0, "seed the RNG (0 = random, time-based)")
		printOnly   = fs.Bool("print", false, "only print the name(s), don't create a directory")
		gitInit     = fs.Bool("git", false, "run 'git init' in the created directory")
		shellInitF  = fs.Bool("shell-init", false, "print the shell function that also cds into the new dir, for eval in ~/.zshrc or ~/.bash_profile")
	)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "novo: create a project dir with a mnemonic name (e.g. %s) and print its path.\n\n", example())
		fmt.Fprintf(stderr, "Usage: novo [flags]\n\nFlags:\n")
		fs.PrintDefaults()
		fmt.Fprintf(stderr, "\nAdd 'eval \"$(novo --shell-init)\"' to your shell profile to also cd into the new directory.\n")
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	if *shellInitF {
		fmt.Fprint(stdout, shellInit)
		return 0
	}

	if *printOnly && *gitInit {
		fmt.Fprintln(stderr, "novo: -git needs a directory, can't combine with -print")
		return 2
	}
	mkdir := !*printOnly

	if *stats {
		fmt.Fprintf(stdout, "%d adjectives x %d nouns = %d combinations\n", len(adjectives), len(nouns), len(adjectives)*len(nouns))
		return 0
	}

	if *clearHist {
		if err := os.Remove(*historyPath); err != nil && !os.IsNotExist(err) {
			fmt.Fprintln(stderr, "novo: clearing history:", err)
			return 1
		}
		fmt.Fprintln(stderr, "novo: history cleared:", *historyPath)
		return 0
	}

	if *count <= 0 {
		fmt.Fprintf(stderr, "novo: -n must be a positive integer, got %d\n", *count)
		return 2
	}

	rngSeed := *seed
	if rngSeed == 0 {
		rngSeed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(rngSeed))

	var history map[string]bool
	if !*noHistory {
		var err error
		history, err = loadHistory(*historyPath)
		if err != nil {
			fmt.Fprintln(stderr, "novo: loading history:", err)
			return 1
		}
	}

	batch := make(map[string]bool)
	var fresh []string

	// Creating a dir needs the fs check on regardless of -no-fscheck: it's
	// the only thing stopping two names from colliding on the same directory.
	checkFs := !*noFsCheck || mkdir

	// On a mid-run failure, still record the names already handed out so
	// a later run won't repeat them.
	code := 0
	for i := 0; i < *count; i++ {
		name, err := pickName(rng, adjectives, nouns, *sep, *suffix, history, batch, *dir, checkFs)
		if err != nil {
			fmt.Fprintln(stderr, "novo:", err)
			code = 1
			break
		}

		if mkdir {
			path, err := createProjectDir(*dir, name, *gitInit, stderr)
			if err != nil {
				fmt.Fprintln(stderr, "novo:", err)
				code = 1
				break
			}
			fmt.Fprintln(stdout, path)
		} else {
			fmt.Fprintln(stdout, name)
		}
		batch[name] = true
		fresh = append(fresh, name)
	}

	// History only helps avoid repeats; failing to save it must not hide
	// directories that were already created, so it's just a warning.
	if !*noHistory && len(fresh) > 0 {
		if err := appendHistory(*historyPath, fresh); err != nil {
			fmt.Fprintln(stderr, "novo: warning: saving history:", err)
		}
	}

	return code
}

// pickName generates a name built from adjs and ns that is not present in
// history or batch, and (if checkFs) not already taken by a file/dir in
// dir. It falls back to a numeric suffix if the name space is exhausted
// after maxPickAttempts tries.
func pickName(rng *rand.Rand, adjs, ns []string, sep string, suffix bool, history, batch map[string]bool, dir string, checkFs bool) (string, error) {
	taken := func(name string) bool {
		if history[name] || batch[name] {
			return true
		}
		if checkFs {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				return true
			}
		}
		return false
	}

	// try draws up to maxPickAttempts random names, with a zero-padded
	// numeric suffix of the given width (0 = no suffix).
	try := func(digits int) (string, bool) {
		for attempt := 0; attempt < maxPickAttempts; attempt++ {
			name := adjs[rng.Intn(len(adjs))] + sep + ns[rng.Intn(len(ns))]
			if digits > 0 {
				name = fmt.Sprintf("%s%s%0*d", name, sep, digits, rng.Intn(int(math.Pow10(digits))))
			}
			if !taken(name) {
				return name, true
			}
		}
		return "", false
	}

	digits := 0
	if suffix {
		digits = 2
	}
	if name, ok := try(digits); ok {
		return name, nil
	}
	// Name space exhausted (or very unlucky): force uniqueness with a wider suffix.
	if name, ok := try(4); ok {
		return name, nil
	}

	return "", fmt.Errorf("could not find a free name after %d attempts (dir=%s)", maxPickAttempts*2, dir)
}

// createProjectDir creates base/name as a new directory (fails if it
// already exists — never silently reuses one) and, if gitInit is set,
// runs 'git init -q' inside it, logging any warning to stderr. It
// returns the directory's absolute path.
func createProjectDir(base, name string, gitInit bool, stderr io.Writer) (string, error) {
	path := filepath.Join(base, name)
	if err := os.Mkdir(path, 0o755); err != nil {
		return "", fmt.Errorf("creating directory: %w", err)
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving absolute path: %w", err)
	}

	if gitInit {
		if _, err := exec.LookPath("git"); err != nil {
			fmt.Fprintln(stderr, "novo: git not found on PATH, skipping 'git init'")
		} else if out, err := exec.Command("git", "-C", abs, "init", "-q").CombinedOutput(); err != nil {
			fmt.Fprintf(stderr, "novo: git init failed: %v\n%s", err, out)
		}
	}

	return abs, nil
}

func example() string {
	return adjectives[0] + "-" + nouns[0]
}

func defaultHistoryPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".novo_history")
	}
	return ".novo_history"
}

func loadHistory(path string) (map[string]bool, error) {
	set := make(map[string]bool)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return set, nil
		}
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			set[line] = true
		}
	}
	return set, scanner.Err()
}

func appendHistory(path string, names []string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, n := range names {
		if _, err := fmt.Fprintln(w, n); err != nil {
			return err
		}
	}
	return w.Flush()
}
