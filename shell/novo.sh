# novo: create a new project dir with a generated name and cd into it.
# A binary can't change its parent shell's cwd, hence this wrapper: it
# cds when the binary printed a single created directory, and otherwise
# (-print, -n 3, -stats, errors, ...) just shows the output.
#
# Enable it with this line in ~/.zshrc or ~/.bash_profile (install.sh adds it):
#   eval "$(novo --shell-init)"
#
# Usage:
#   novo                                  # create ./quiet-falcon and cd into it
#   novo --git                            # same, plus git init
#   novo --dir ~/Projects                 # create the project under ~/Projects
#   novo --print -n 5                     # only print 5 names
novo() {
	local out rc
	out=$(command novo "$@")
	rc=$?
	case $out in
	/*) [ "$rc" -eq 0 ] && [ -d "$out" ] && cd "$out" && { echo "$out"; return; } ;;
	esac
	[ -z "$out" ] || printf '%s\n' "$out"
	return "$rc"
}
