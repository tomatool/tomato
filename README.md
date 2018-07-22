# gebet - behavioral testing tools

Behavior Driven Development tools, built on top of (https://github.com/DATA-DOG/godog). To simplify adding BDD to your application without writing any code.

gebet uses yaml config file to specify [resources](#resources)

## Resources

Resource are the objects that are going to be used for steps in the scenario. They are listed under the resources key in the pipeline configuration.

### db/sql

**Required parameters**
- driver: either `postgres` or `mysql`
- datasource: database source name

**Available functions**

1. **Set**

      Set database values, example:
      
        Given set "db1" table "customers" list of content
          | name    | country |
          | cembri  | us      |
        
    
2. **Compare**

      Compare database values, example:
      
        Then "db1" table "customers" should look like
          | name    |
          | cembri  |
    
    
      **Notes**: missing fields will be compromised
   
### http/client

standard http client, for sending http request

**Optional parameters**
- base_url: base url for all request using this http client
- timeout: set timeout for request using this client, passing timeout will cause test to fail

**Available functions**

1. **Send request**

      Send request with body, example:
              
        Then "httpcli" send request to "POST /api/v1/customers" with body
            """
                {
                    "name": "ali",
                    "country": "id"
                }
            """
    **Notes**: adding `base_url` as params, will make this task send request to `$(base_url)/api/v1/customers`
        
2. **Check Response Code**

      Compare response code, example:
      
        Then "httpcli" response code should be 201
    
3. **Check Response Body**

    Compare response code, example:
    
        Then "httpcli" response body should be
          """
            {"status":"OK"}
          """
    **Notes**: missing fields will be compromised
    
### http/server

standard http server, for mocking external server response

**Required parameters**
- port: address that going to be used to be the server, example `:8001`

**Available functions**

1. **Set Response**

  Send request with body, example:
  
    Given set "external-service-a" response code to 200 and response body
        """
            {
                "country": "id",
                "status": "ok"
            }
        """

## Example
```sh
$ gebet -c examples/config.yaml -f examples/features/
.F- 3


--- Failed steps:

  Scenario: Send test request # examples/features/test.feature:3
    Then "http" response code should be 400 # examples/features/test.feature:15
      Error:
          [MISMATCH] response code
          expecting	:	400
          got		:	204


1 scenarios (1 failed)
3 steps (1 passed, 1 failed, 1 skipped)
4.628464ms

Randomized with seed: 1532084250162704720
```
