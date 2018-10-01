# Message Queue

Initialize resource in `config.yml`:
```yaml
resources:
  - name: # name of the resource
    type: queue
    ready_check: true
    params:
      driver: # driver
      datasource: # data source name of the message queue
```

* RabbitMQ
```yaml
params:
  driver: rabbitmq
  datasource: amqp://guest:guest@rabbit:5672
```

* NSQ
```yaml
params:
  driver: nsq
  datasource: nsqd:4150
```

## Glossary
* **target**
  * RabbitMQ: `[exchange]:[routing-key]`
  * NSQ: `[topic]`


## Actions
* Publish - to publish message to message queue
```gherkin
Given publish message to "$resourceName" target "$target" with payload body
"""
  $messageBody
"""
```

* Listen - to listen messages from a given target
```gherkin
Given listen message from "$resourceName" target "$target"
```

* Count - to count messages from a target (listen is required)
```gherkin
Then message from "$resourceName" target "$target" count should be $count
```

* Compare - to check consumed message payload from
```gherkin
Then message from "$resourceName" target "$target" should look like
"""
  $messageBody
"""
```
