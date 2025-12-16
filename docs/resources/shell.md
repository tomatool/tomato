# Shell

Steps for executing shell commands and scripts


## Setup

| Step | Description |
|------|-------------|
| `"shell" env "API_KEY" is "secret"` | Set environment variable |
| `"shell" workdir is "/tmp/test"` | Set working directory |


## Execution

| Step | Description |
|------|-------------|
| `"shell" runs:` | Run command (docstring) |
| `"shell" runs "ls -la"` | Run inline command |
| `"shell" runs script "scripts/setup.sh"` | Run script file |
| `"shell" runs with timeout "60s":` | Run with custom timeout |


## Exit Code

| Step | Description |
|------|-------------|
| `"shell" exit code is "0"` | Assert exit code |
| `"shell" succeeds` | Assert exit code 0 |
| `"shell" fails` | Assert non-zero exit code |


## Output

| Step | Description |
|------|-------------|
| `"shell" stdout contains "success"` | Assert stdout contains substring |
| `"shell" stdout does not contain "error"` | Assert stdout doesn't contain |
| `"shell" stdout is:` | Assert exact stdout |
| `"shell" stdout is empty` | Assert stdout empty |
| `"shell" stderr contains "warning"` | Assert stderr contains substring |
| `"shell" stderr is empty` | Assert stderr empty |


## Files

| Step | Description |
|------|-------------|
| `"shell" file "output.txt" exists` | Assert file exists |
| `"shell" file "temp.txt" does not exist` | Assert file doesn't exist |
| `"shell" file "config.json" contains "database"` | Assert file contains substring |

