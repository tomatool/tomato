# HTTP Client

Initialize resource in `config.yml`:
```yaml
resources:
  - name: # name of the resource
    type: http/client
    ready_check: false
    params:
      base_url: # application based url
```

## Glossary
* **target** - endpoint composed by request method & target separated by space

  Example:
  * `GET /hello/world`
  * `POST http://google.com/hello/world`

## Actions
* Send Request - to send http request
```gherkin
Given "$resourceName" send request to "$target"
```

* Send Request with Body - to send http request with request body
```gherkin
Given "$resourceName" send request to "$target" with body
"""
    $requestBody
"""
```
```gherkin
Given "$resourceName" send request to "$target" with payload
"""
    $requestBody
"""
```

* Check response code - to check response code of request that been made
```gherkin
Then "$resourceName" response code should be $statusCode
```

* Check response header - to check header values of to a request that has been made

```gherkin
Then "$resourceName" repsonse header "$headerName" should be "$headerValue"
```

* Check response body - to check response body of request that been made
```gherkin
Then "$resourceName" response body should contain
"""
  $responseBody
"""
```
