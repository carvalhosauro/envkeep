#!/usr/bin/env sh
# Records the README gifs with vhs (https://github.com/charmbracelet/vhs).
#
#   ./demo/record.sh          # record every tape in demo/tapes/
#   ./demo/record.sh use      # record just demo/tapes/use.tape
#
# Each tape gets a fresh demo repo (setup-demo-repo.sh) so recordings are
# reproducible. Gifs land in demo/*.gif. Requires vhs, ttyd, and ffmpeg.
set -e

cd "$(dirname "$0")/.."
make build
PATH="$PWD/bin:$PATH"
export PATH

command -v vhs >/dev/null 2>&1 || { echo "vhs not found: go install github.com/charmbracelet/vhs@latest" >&2; exit 1; }
command -v ttyd >/dev/null 2>&1 || { echo "ttyd not found: https://github.com/tsl0922/ttyd/releases" >&2; exit 1; }
command -v ffmpeg >/dev/null 2>&1 || { echo "ffmpeg not found" >&2; exit 1; }

if [ $# -ge 1 ]; then
	tapes="demo/tapes/$1.tape"
	[ -f "$tapes" ] || { echo "no such tape: $tapes" >&2; exit 1; }
else
	tapes=$(ls demo/tapes/*.tape)
fi

for tape in $tapes; do
	echo "==> $tape"
	sh demo/setup-demo-repo.sh >/dev/null
	vhs "$tape"
done

echo "done. gifs:"
ls -lh demo/*.gif
