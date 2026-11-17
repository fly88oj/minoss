#!/bin/bash

function download_arch_specific_executable {
	curl -f -L -s -q \
		https://github.com/moparisthebest/static-curl/releases/latest/download/curl-$1 \
		-o /go/bin/curl || exit 1
	chmod +x /go/bin/curl
}

case $TARGETARCH in
"arm64")
	# moparisthebest/static-curl dropped the aarch64 asset in v8.17.0; v8.11.0
	# is the last release carrying curl-aarch64. Probe-only binary, so a pinned
	# asset beats the 'latest' link that now 404s (run 44074926767, arm64 leg).
	curl -f -L -s -q \
		https://github.com/moparisthebest/static-curl/releases/download/v8.11.0/curl-aarch64 \
		-o /go/bin/curl || exit 1
	chmod +x /go/bin/curl
	;;
"s390x")
	echo "Not downloading static cURL because it does not exist for the $TARGETARCH architecture."
	# Dockerfile.release unconditionally COPYs /go/bin/curl* into the runtime
	# image; leave a matching placeholder so the glob does not fail the build.
	: > /go/bin/curl.not-available
	;;
*)
	download_arch_specific_executable "$TARGETARCH"
	;;
esac
