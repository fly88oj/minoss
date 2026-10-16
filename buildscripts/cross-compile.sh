#!/bin/bash

set -e
# Enable tracing if set.
[ -n "$BASH_XTRACEFD" ] && set -x

function _init() {
	## All binaries are static make sure to disable CGO.
	export CGO_ENABLED=0

	## List of architectures and OS to test cross compilation.
	## 32-bit targets (linux/386, linux/arm, linux/mips) are gone: the
	## dependency refresh to aead.dev/mtls v0.4.0 broke 32-bit builds
	## (`math.MaxUint32` overflows int) and no newer mtls release exists;
	## downgrading a crypto dependency to keep 32-bit is the wrong trade.
	## If 32-bit ever matters again, fork-patch mtls key.go, then re-add
	## the targets here and in release.yml. netbsd/amd64 stays: its
	## go-systemd v22.7.0 breakage is fixed by the replace pin in go.mod.
	SUPPORTED_OSARCH="linux/ppc64le linux/mips64 linux/amd64 linux/arm64 linux/s390x darwin/arm64 darwin/amd64 freebsd/amd64 windows/amd64 netbsd/amd64 openbsd/amd64 linux/riscv64"
}

function _build() {
	local osarch=$1
	local os arch
	IFS=/ read -r os arch <<<"$osarch"
	printf -- "--> %15s:%s\n" "${osarch}" "${PACKAGE}"

	# go build -trimpath to build the binary. The import path is invariant
	# across targets, so it is resolved once in main() instead of once per
	# platform.
	GOOS=$os GOARCH=$arch GO111MODULE=on go build -trimpath -o /dev/null .
}
export -f _build

function main() {
	PACKAGE=$(go list -f '{{.ImportPath}}')
	export PACKAGE

	echo "Testing builds for OS/Arch: ${SUPPORTED_OSARCH}"
	# The platforms are independent compiles; run them concurrently instead
	# of paying 15 sequential cold-link round trips. Parallelism is capped:
	# each go build can hold ~1-2GB, and an unbounded -P $(nproc) on a big
	# host starves anything else running there. xargs exits non-zero if any
	# build fails.
	local maxjobs
	maxjobs=$(( $(nproc) > 8 ? 8 : $(nproc) ))
	printf '%s\n' ${SUPPORTED_OSARCH} | xargs -n1 -P "${maxjobs}" -I{} bash -c '_build "$1"' _ {}
}

_init && main "$@"
