#!/usr/bin/env bash
# Builds the coverage report posted on pull requests, and enforces the gates.
#
# Inputs (written by `make coverage`):
#   coverage/coverage.out   merged unit + integration profile (textfmt)
#   coverage/steps.json     `tomato coverage --format json --all-types`
#   coverage/integration.exit   exit code of the integration suite
#
# Gates:
#   - every resource type's step coverage must be 100%
#   - Go statement coverage of internal/handler (the resources) must be at
#     least the value in .coverage-min (handler=...), and the whole module at
#     least total=...
#   - the integration suite must pass
#
# Writes coverage/report.md and exits non-zero when a gate fails.
set -euo pipefail

out=coverage/report.md
profile=coverage/coverage.out
steps=coverage/steps.json
module=github.com/tomatool/tomato

min_handler=$(sed -n 's/^handler=//p' .coverage-min 2>/dev/null || echo 0)
min_total=$(sed -n 's/^total=//p' .coverage-min 2>/dev/null || echo 0)

# Statement coverage per file: coverage.out lines are
#   file:startLine.col,endLine.col numStatements hitCount
per_file() {
  tail -n +2 "$profile" | awk '
    {
      split($1, a, ":"); f = a[1]; n = $2; hit = ($3 > 0)
      key = f SUBSEP $1
      if (!(key in seen)) { seen[key] = 1; total[f] += n; if (hit) cov[f] += n; covered[key] = hit }
      else if (hit && !covered[key]) { covered[key] = 1; cov[f] += n }
    }
    END { for (f in total) printf "%s %d %d\n", f, cov[f], total[f] }'
}

pct() { awk -v c="$1" -v t="$2" 'BEGIN { if (t == 0) print "100.0"; else printf "%.1f", 100 * c / t }'; }

files=$(per_file | sort)

sum_prefix() {
  echo "$files" | awk -v p="$1" 'index($1, p) == 1 { c += $2; t += $3 } END { printf "%d %d\n", c, t }'
}

read -r total_c total_t < <(sum_prefix "$module/")
read -r handler_c handler_t < <(sum_prefix "$module/internal/handler/")
total_pct=$(pct "$total_c" "$total_t")
handler_pct=$(pct "$handler_c" "$handler_t")

step_total=$(jq '.total' "$steps")
step_covered=$(jq '.covered' "$steps")
step_pct=$(pct "$step_covered" "$step_total")
steps_below=$(jq -r '[.types[] | select(.covered < .total) | .type] | join(", ")' "$steps")

integration_exit=$(cat coverage/integration.exit 2>/dev/null || echo 1)

fail=0
gate() { # name ok detail
  if [ "$2" = 1 ]; then echo "| ✅ | $1 | $3 |"; else echo "| ❌ | $1 | $3 |"; fail=1; fi
}
ge() { awk -v a="$1" -v b="$2" 'BEGIN { print (a + 0 >= b + 0) ? 1 : 0 }'; }

{
  echo "<!-- tomato-coverage-report -->"
  echo "## 🍅 Coverage"
  echo
  echo "| | Gate | Result |"
  echo "|---|---|---|"
  gate "Integration suite passes" "$([ "$integration_exit" = 0 ] && echo 1 || echo 0)" "exit code $integration_exit"
  gate "Every resource step used by a scenario" "$([ -z "$steps_below" ] && echo 1 || echo 0)" "$step_covered / $step_total steps ($step_pct%)${steps_below:+ — below 100%: $steps_below}"
  gate "Resource code coverage (\`internal/handler\`) ≥ $min_handler%" "$(ge "$handler_pct" "$min_handler")" "$handler_pct%"
  gate "Total code coverage ≥ $min_total%" "$(ge "$total_pct" "$min_total")" "$total_pct%"
  echo
  echo "Code coverage is unit tests and the integration suite (\`tests/tomato.yml\`) merged."
  echo
  echo "### Step coverage by resource"
  echo
  jq -r '
    "| Resource | Steps | Coverage |",
    "|---|---|---|",
    (.types[] | "| \(.name) `\(.type)` | \(.covered) / \(.total) | \(if .covered == .total then "✅" else "❌" end) \(if .total == 0 then 100 else (1000 * .covered / .total | floor) / 10 end)% |")
  ' "$steps"
  missing=$(jq -r '.types[] | .type as $t | .uncovered[]? | "- `\($t)`: `\(.)`"' "$steps")
  if [ -n "$missing" ]; then
    echo
    echo "<details><summary>Steps no scenario uses</summary>"
    echo
    echo "$missing"
    echo
    echo "</details>"
  fi
  echo
  echo "<details><summary>Code coverage by resource file (<code>internal/handler</code>)</summary>"
  echo
  echo "| File | Statements | Coverage |"
  echo "|---|---|---|"
  echo "$files" | awk -v p="$module/internal/handler/" 'index($1, p) == 1 && $1 !~ /_test\.go$/ {
      f = substr($1, length(p) + 1); printf "| `%s` | %d / %d | %.1f%% |\n", f, $2, $3, ($3 ? 100 * $2 / $3 : 100) }'
  echo
  echo "</details>"
  echo
  echo "<details><summary>Code coverage by package</summary>"
  echo
  echo "| Package | Statements | Coverage |"
  echo "|---|---|---|"
  echo "$files" | awk -v m="$module/" '{
      f = substr($1, length(m) + 1); n = split(f, parts, "/"); pkg = (n > 1) ? substr(f, 1, length(f) - length(parts[n]) - 1) : "."
      c[pkg] += $2; t[pkg] += $3 }
    END { for (p in t) printf "| `%s` | %d / %d | %.1f%% |\n", p, c[p], t[p], (t[p] ? 100 * c[p] / t[p] : 100) }' | sort
  echo
  echo "</details>"
} > "$out"

cat "$out"
exit "$fail"
