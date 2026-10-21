# Redis

Steps for interacting with Redis key-value store

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## String Operations

| Step | Description |
|------|-------------|
| `"app-cache" key "user:1" is "John"` | Set a string value |
| `"app-cache" key "session:abc" is "data" with TTL "1h"` | Set a string value with expiration |
| `"app-cache" key "user:1" is:` | Set a JSON/multiline value |
| `"app-cache" key "user:1" is deleted` | Delete a key |
| `"app-cache" key "counter" is incremented` | Increment integer value by 1 |
| `"app-cache" key "counter" is incremented by "5"` | Increment integer value by N |
| `"app-cache" key "counter" is decremented` | Decrement integer value by 1 |



## String Assertions

| Step | Description |
|------|-------------|
| `"app-cache" key "user:1" exists` | Assert key exists |
| `"app-cache" key "user:1" does not exist` | Assert key doesn't exist |
| `"app-cache" key "user:1" has value "John"` | Assert exact value |
| `"app-cache" key "user:1" contains "John"` | Assert value contains substring |
| `"app-cache" key "session:abc" has TTL greater than "3600" seconds` | Assert TTL greater than N seconds |



## Hash Operations

| Step | Description |
|------|-------------|
| `"app-cache" hash "user:1" has fields:` | Set hash fields from table |
| `"app-cache" hash "user:1" field "name" is "John"` | Assert hash field value |
| `"app-cache" hash "user:1" contains:` | Assert hash contains fields |



## List Operations

| Step | Description |
|------|-------------|
| `"app-cache" list "queue" has "item1"` | Push value to list |
| `"app-cache" list "queue" has values:` | Push multiple values to list |
| `"app-cache" list "queue" has "3" items` | Assert list length |
| `"app-cache" list "queue" contains "item1"` | Assert list contains value |



## Set Operations

| Step | Description |
|------|-------------|
| `"app-cache" set "tags" has "tag1"` | Add member to set |
| `"app-cache" set "tags" has members:` | Add multiple members to set |
| `"app-cache" set "tags" contains "tag1"` | Assert set contains member |
| `"app-cache" set "tags" has "3" members` | Assert set size |



## Database

| Step | Description |
|------|-------------|
| `"app-cache" has "5" keys` | Assert total key count |
| `"app-cache" is empty` | Assert database is empty |


