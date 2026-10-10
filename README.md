# novo

Creates a new project directory with a mnemonic, docker-style name
(`quiet-falcon`, `fierce-otter-07`) and cds into it — so you never spend
time coming up with or agreeing on names for new projects/repos/directories.

Works on macOS and Linux, in zsh (the macOS default) and bash. `--git`
requires `git`.

## Install

Open **Terminal** and paste:

```sh
curl -fsSL https://raw.githubusercontent.com/qomyagin/novo/main/install.sh | sh
```

Then open a new Terminal window and type `novo`. That's it — no Go or
developer tools needed.

The [installer](install.sh) downloads the prebuilt `novo` for your Mac
(Apple Silicon or Intel) into `~/.local/bin` and adds two lines to
`~/.zshrc` (or `~/.bash_profile` for bash). Run it again to update.

To uninstall: `rm ~/.local/bin/novo ~/.novo_history` and delete the
`# novo` lines from that profile file.

### From source

```sh
git clone https://github.com/qomyagin/novo.git && cd novo
make install                                         # tests, builds, copies to ~/.local/bin
echo 'eval "$(novo --shell-init)"' >> ~/.zshrc       # or ~/.bash_profile
```

`go install github.com/qomyagin/novo@latest` works too; add the same
`eval` line to get auto-cd.

Why the `eval` line: a program can't change its parent shell's directory,
so `novo --shell-init` prints a small `novo` shell function
([`shell/novo.sh`](shell/novo.sh)) that runs the program and cds into the
directory it created.

### Releasing

Push a tag (`git tag v0.1.0 && git push --tags`);
[`release.yml`](.github/workflows/release.yml) builds the macOS/Linux
binaries the installer downloads.

## Usage

```sh
novo                       # create ./quiet-falcon and cd into it
novo --git                 # same, plus git init
novo --dir ~/Projects      # create it under ~/Projects instead
novo -n 3                  # create 3 directories (no cd)

novo --print               # only print a name: quiet-falcon
novo --print -n 5          # print 5 names
novo --suffix              # with a numeric suffix: fond-deer-53
novo --sep _               # different separator: fond_deer
novo --no-fscheck          # with --print: skip the filesystem check
novo --no-history          # don't track/check the history file
novo --clear-history       # clear history
novo --stats               # name-space size
```

Without the shell function (e.g. in scripts) the binary still creates the
directory and prints its absolute path: `cd "$(novo)"`.

## How it works

- Name = `<adjective>-<noun>` (word lists in `words.go`,
  176 × 180 ≈ 31k combinations).
- By default, a name is checked against:
  - repeats in history (`~/.novo_history`, one name per line, appended
    automatically);
  - an existing file/dir with the same name in the current directory (or
    `--dir`).
- If no free name is found after 500 attempts, a 4-digit random suffix is
  appended.
- The directory is created with `os.Mkdir` (never reuses an existing one)
  and its absolute path is printed; `--git` additionally runs
  `git init -q` inside it.

## Testing & coverage

```sh
make test         # go test ./... -race
make cover        # same, plus a coverage report gated at 80% minimum
make cover-html   # cover + an HTML report at coverage.html
```

`make cover` fails the build if total statement coverage drops below
`COVER_MIN` (default 80%, the common baseline for Go projects) — override
with `make cover COVER_MIN=90`. CI (`.github/workflows/ci.yml`) runs
`gofmt`, `go vet` and `make cover` on every push/PR.

The CLI logic lives in `run(args, stdout, stderr)`, which `main` just
calls and forwards the exit code for — this keeps `flag` state local per
call instead of global, so tests can invoke the CLI repeatedly with
different arguments without subprocesses.

## Files

- `main.go` — CLI, generation, history, fs-check.
- `main_test.go` — unit and CLI-level tests.
- `words.go` — adjective and noun word lists.
- `shell/novo.sh` — the `novo` shell function that cds (embedded, printed by `--shell-init`).
- `install.sh` — one-line installer from GitHub Releases.
- `Makefile` — build/install/uninstall/test/cover.
- `.github/workflows/ci.yml` — CI: fmt, vet, tests, coverage gate.
- `.github/workflows/release.yml` — builds release binaries on a `v*` tag.
- `LICENSE` — MIT.

## License

[MIT](LICENSE).
