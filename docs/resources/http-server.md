# HTTP Client

Initialize resource in `config.yml`:
```yaml
resources:
  - name: # name of the resource
    type: http/server
    ready_check: false
    params:
      port: # server port
```

## Actions
* Set response - to send http request
```gherkin
Given set "$resourceName" with path "$path" response code to $responseCode and response body
"""
  $responseBody
"""
```
