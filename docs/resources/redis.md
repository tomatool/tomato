# Redis

Steps for interacting with Redis key-value store

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## String Operations

| Step | Description |
|------|-------------|
| `"cache" key "user:1" is "John"` | Set a string value |
| `"cache" key "session:abc" is "data" with TTL "1h"` | Set a string value with expiration |
| `"cache" key "user:1" is:` | Set a JSON/multiline value |
| `"cache" key "user:1" is deleted` | Delete a key |
| `"cache" key "counter" is incremented` | Increment integer value by 1 |
| `"cache" key "counter" is incremented by "5"` | Increment integer value by N |
| `"cache" key "counter" is decremented` | Decrement integer value by 1 |



## String Assertions

| Step | Description |
|------|-------------|
| `"cache" key "user:1" exists` | Assert key exists |
| `"cache" key "user:1" does not exist` | Assert key doesn't exist |
| `"cache" key "user:1" has value "John"` | Assert exact value |
| `"cache" key "user:1" contains "John"` | Assert value contains substring |
| `"cache" key "session:abc" has TTL greater than "3600" seconds` | Assert TTL greater than N seconds |



## Hash Operations

| Step | Description |
|------|-------------|
| `"cache" hash "user:1" has fields:` | Set hash fields from table |
| `"cache" hash "user:1" field "name" is "John"` | Assert hash field value |
| `"cache" hash "user:1" contains:` | Assert hash contains fields |



## List Operations

| Step | Description |
|------|-------------|
| `"cache" list "queue" has "item1"` | Push value to list |
| `"cache" list "queue" has values:` | Push multiple values to list |
| `"cache" list "queue" has "3" items` | Assert list length |
| `"cache" list "queue" contains "item1"` | Assert list contains value |



## Set Operations

| Step | Description |
|------|-------------|
| `"cache" set "tags" has "tag1"` | Add member to set |
| `"cache" set "tags" has members:` | Add multiple members to set |
| `"cache" set "tags" contains "tag1"` | Assert set contains member |
| `"cache" set "tags" has "3" members` | Assert set size |



## Database

| Step | Description |
|------|-------------|
| `"cache" has "5" keys` | Assert total key count |
| `"cache" is empty` | Assert database is empty |


