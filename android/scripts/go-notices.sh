#!/usr/bin/env bash
# Prints the license of Go and of every module linked into the Go core, for
# the app's Acerca de screen. make android-core runs it with the core's GOOS
# and GOARCH and writes app/src/main/assets/licenses/go-modules.txt.
set -euo pipefail

echo "The Go core of cliamp-x (libcliamp.so) is built with Go and links these modules."
printf '\n== Go %s ==\n\n' "$(go env GOVERSION)"
cat "$(go env GOROOT)/LICENSE"
go list -deps -f '{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}} {{.Dir}}{{end}}{{end}}' . | sort -u |
	while read -r path version dir; do
		printf '\n== %s %s ==\n\n' "$path" "$version"
		file=$(find "$dir" -maxdepth 1 -type f -iregex '.*/\(un\)?\(licen[cs]e\|copying\)[^/]*' | sort | head -1)
		if [[ -n $file ]]; then cat "$file"; else echo "(The module has no license file: see its source.)"; fi
	done
