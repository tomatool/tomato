## Resources

Resource are the objects that are going to be used for steps in the scenario. They are listed under the resources key in the pipeline configuration.

### List of available resources

* [Database SQL](https://github.com/tomatool/tomato/blob/main/docs/resources.md#database-sql) - to manage (insert/check) database state changes
  * [MySQL](https://github.com/tomatool/tomato/blob/main/docs/resources.md#database-sql)
  * [PostgreSQL](https://github.com/tomatool/tomato/blob/main/docs/resources.md#database-sql)
* [HTTP Client](https://github.com/tomatool/tomato/blob/main/docs/resources.md#http-client) - to send HTTP Request
* [HTTP Server](https://github.com/tomatool/tomato/blob/main/docs/resources.md#http-server) - to mock external HTTP Server
  * [Wiremock](https://github.com/tomatool/tomato/blob/main/docs/resources.md#http-server)
* [Message Queue](https://github.com/tomatool/tomato/blob/main/docs/resources.md#queue) - to manage (publish/consume/check) message queue
  * [RabbitMQ](https://github.com/tomatool/tomato/blob/main/docs/resources.md#queue)
  * [NSQ](https://github.com/tomatool/tomato/blob/main/docs/resources.md#queue)
* [Shell](https://github.com/tomatool/tomato/blob/main/docs/resources.md#shell) -  to execute unix shell command-line interface
* [Cache](https://github.com/tomatool/tomato/blob/main/docs/resources.md#cache)
  * [Redis](https://github.com/tomatool/tomato/blob/main/docs/resources.md#cache)
