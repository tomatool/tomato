# Resources

Resource are the objects that are going to be used for steps in the scenario. They are listed under the resources key in the pipeline configuration.

## List of available resources

* [Database SQL](https://alileza.github.io/tomato/resources/database-sql) - to manage (insert/check) database state changes
  * [MySQL](https://alileza.github.io/tomato/resources/database-sql)
  * [PostgreSQL](https://alileza.github.io/tomato/resources/database-sql)
* [HTTP Client](https://alileza.github.io/tomato/resources/http-client) - to send HTTP Request
* [HTTP Server](https://alileza.github.io/tomato/resources/http-server) - to create fake external service dependency that serve HTTP API
* [Message Queue](https://alileza.github.io/tomato/resources/message-queue) - to manage (publish/consume/check) message queue
  * [RabbitMQ](https://alileza.github.io/tomato/resources/message-queue)
  * [NSQ](https://alileza.github.io/tomato/resources/message-queue)
