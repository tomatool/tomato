# Resources

Resource are the objects that are going to be used for steps in the scenario. They are listed under the resources key in the pipeline configuration.

List of available resource:

* database
    - [sql](#sql)
* http
    - [client](#httpclient)
    - [server](#httpserver)
* [queue](#queue)

---

# Database
## SQL
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
      
---

# HTTP
## Client

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

## Server


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

---

# Queue

consume & publish from message queue. target stands for `[exchange]:[key]`

**Required parameters**
- driver: in the meantime it only support `rabbitmq`
- datasource: queue source name

**Available functions**

1. **Listen**

      Listen to some queue, that you want to check in the future steps

          Then listen message from "my-awesome-queue" target "customers:uyeah"


  **Notes**: Listen will also purged all message in the queue before start listening.
2. **Publish**

      Publish queue

        Then publish message to "my-awesome-queue" target "customers:uyeah" with payload
            """
                {
                    "test":"OK"
                }
            """

3. **Count**

      Count message in the queue

        Then message from "my-awesome-queue" target "customers:uyeah" count should be 2


4. **Compare**

      Compare consumed message from the queue

        Then message from "my-awesome-queue" target "customers:uyeah" should look like
            """
                {
                    "test":"OK"
                }
            """
