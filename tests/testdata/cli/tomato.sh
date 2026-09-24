#!/bin/sh
# Runs tomato for tests/features/cli.feature and strips ANSI colors from
# stdout so steps can match plain text. Keeps the exit code and stderr.
# TOMATO_BIN lets `make coverage` use the coverage-instrumented binary.
out=$("${TOMATO_BIN:-./bin/tomato}" "$@")
rc=$?
printf '%s\n' "$out" | sed "s/$(printf '\033')\[[0-9;]*m//g"
exit $rc
