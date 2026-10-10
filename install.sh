#!/bin/sh
# Installs novo: downloads the latest release binary into ~/.local/bin and
# enables the shell function (auto-cd) in your shell profile.
#
#   curl -fsSL https://raw.githubusercontent.com/qomyagin/novo/main/install.sh | sh
set -eu

repo=qomyagin/novo
bin_dir="$HOME/.local/bin"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case $os in
darwin | linux) ;;
*) echo "novo: unsupported OS: $os" >&2; exit 1 ;;
esac

case $(uname -m) in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*) echo "novo: unsupported CPU: $(uname -m)" >&2; exit 1 ;;
esac

mkdir -p "$bin_dir"
curl -fsSL "https://github.com/$repo/releases/latest/download/novo-$os-$arch" -o "$bin_dir/novo.tmp"
chmod +x "$bin_dir/novo.tmp"
mv "$bin_dir/novo.tmp" "$bin_dir/novo"

case ${SHELL##*/} in
zsh) profile="$HOME/.zshrc" ;;
bash) if [ "$os" = darwin ]; then profile="$HOME/.bash_profile"; else profile="$HOME/.bashrc"; fi ;;
*) profile="$HOME/.profile" ;;
esac

if ! grep -qF 'novo --shell-init' "$profile" 2>/dev/null; then
	printf '\n# novo: create a project dir with a mnemonic name and cd into it\nexport PATH="$HOME/.local/bin:$PATH"\neval "$(novo --shell-init)"\n' >>"$profile"
fi

echo "novo installed to $bin_dir/novo (enabled in $profile)."
echo "Open a new terminal window and run: novo"
