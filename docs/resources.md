# Resources

Resource are the objects that are going to be used for steps in the scenario. They are listed under the resources key in the pipeline configuration.

## HTTP Client

http client that can be used for sending http requests and comparing the responses

Initialize resource in `config.yml`:
```yaml
- name: # name of the resource
  type: # | httpclient | 
  
  options:
    # base url for the http client that will automatically be prepended to any route in the feature.
    base_url: # string
    # timeout for the request round-trip.
    timeout: # duration
    # stubs file path (`./stubs`)
    stubs_path: # string
    
  
```

### Resources

* httpclient


### Actions

#### **Send**
send an http request without a request body
```gherkin
Given $resource send request to $target

```

#### **Send Body**
send an http request with a request body
```gherkin
Given $resource send request to $target with body $body
Given $resource send request to $target with payload $body

```

#### **Send Body From File**
send an http request with a request body from a file
```gherkin
Given $resource send request to $target with body from file $file
Given $resource send request to $target with payload from file $file

```

#### **Request Header**
set request header
```gherkin
Given $resource set request header key $key with value $value

```

#### **Response Code**
check http response code
```gherkin
Given $resource response code should be $code

```

#### **Response Header**
check http response headers
```gherkin
Given $resource response header $header_name should be $header_value

```

#### **Response Body Contains**
check response body contains a given json fields
```gherkin
Given $resource response body should contain $body

```

#### **Response Body Equals**
check response body to be equals a given json
```gherkin
Given $resource response body should equal $body

```



## HTTP Server

http wiremock server resource that mocks API responses

Initialize resource in `config.yml`:
```yaml
- name: # name of the resource
  type: # | wiremock | 
  
  options:
    # wiremock base url (e.g : http://localhost:8080)
    base_url: # string
    # stubs file path (`./stubs`)
    stubs_path: # string
    
  
```

### Resources

* wiremock


### Actions

#### **Response**
set a response code and body for any request that comes to the wiremock target
```gherkin
Given set $resource response code to $code and response body $body

```

#### **Response From File**
set a response code and body from a file for any request that comes to the wiremock target
```gherkin
Given set $resource response code to $code and response body from file $file

```

#### **Response Path**
set a response code and body for a given path for wiremock
```gherkin
Given set $resource with path $path response code to $code and response body $body

```

#### **Response Method Path**
set a response code and body for a given method and path for wiremock
```gherkin
Given set $resource with method $method and path $path response code to $code and response body $body

```

#### **Response Code Method Path**
set a response code for a given method and path for wiremock
```gherkin
Given set $resource with method $method and path $path response code to $code

```

#### **Response Code Method Path From File**
set a response code and body for a given method and path for wiremock (reads from given file path)
```gherkin
Given set $resource with method $method and path $path response code to $code and response body from file $file

```

#### **Verify Requests**
check requests count on a given endpoint
```gherkin
Given $resource with path $path request count should be $count

```



## Database SQL

database driver that interacts with a sql database

Initialize resource in `config.yml`:
```yaml
- name: # name of the resource
  type: # | postgres | mysql | 
  
  options:
    # sql database source name (`postgres://user:pass@host:port/dbname?sslmode=disable`)
    datasource: # string
    
  
```

### Resources

* postgres
* mysql


### Actions

#### **Set**
truncates the target table and sets row results to the passed values
```gherkin
Given set $resource table $table list of content $content

```

#### **Check**
compares table content after an action
```gherkin
Given $resource table $table should look like $content

```



## Queue

messaging queue that that publishes and serves messages

Initialize resource in `config.yml`:
```yaml
- name: # name of the resource
  type: # | rabbitmq | nsq | 
  
  options:
    # queue source dsn (`amqp://user:pass@host:port/`)
    datasource: # string
    # stubs file path (`./stubs`)
    stubs_path: # string
    
  
```

### Resources

* rabbitmq
* nsq


### Actions

#### **Publish**
publish a message to message queue
```gherkin
Given publish message to $resource target $target with payload $payload

```

#### **Publish From File**
publish a message to message queue from a file
```gherkin
Given publish message to $resource target $target with payload from file $file

```

#### **Listen**
listen for messages on a given queue. Declaration should be before the publish action

```gherkin
Given listen message from $resource target $target

```

#### **Count**
count messages for a given target. Declaration should be before the publish action
```gherkin
Given message from $resource target $target count should be $count

```

#### **CompareContains**
compare message payload by checking if the message contains other JSON. Declaration should be before the publish action
```gherkin
Given message from $resource target $target should contain $payload

```

#### **CompareEquals**
compare message payload by checking for exact JSON matches. Declaration should be before the publish action
```gherkin
Given message from $resource target $target should equal $payload

```



## Shell

to communicate with shell command

Initialize resource in `config.yml`:
```yaml
- name: # name of the resource
  type: # | shell | 
  
  options:
    # shell command prefixes
    prefix: # string
    
  
```

### Resources

* shell


### Actions

#### **Execute**
execute shell command
```gherkin
Given $resource execute $command

```

#### **Stdout Contains**
check stdout for executed command contains a given value
```gherkin
Given $resource stdout should contains $substring

```

#### **Stdout Not Contains**
check stdout for executed command not contains a given value
```gherkin
Given $resource stdout should not contains $substring

```

#### **Stderr Contains**
check stderr for executed command contains a given value
```gherkin
Given $resource stderr should contains $substring

```

#### **Stderr Not Contains**
check stderr for executed command not contains a given value
```gherkin
Given $resource stderr should not contains $substring

```

#### **Exit Code Equal**
check exit code is equal given value
```gherkin
Given $resource exit code equal to $exit_code

```

#### **Exit Code Not Equal**
check exit code is not equal given value
```gherkin
Given $resource exit code not equal to $exit_code

```



## Cache

cache driver that interacts with a cache service

Initialize resource in `config.yml`:
```yaml
- name: # name of the resource
  type: # | redis | 
  
  options:
    # cache driver (only "redis" for now)
    driver: # string
    # cache source url (`redis://user:secret@localhost:6379/0?foo=bar&qux=baz`)
    datasource: # string
    
  
```

### Resources

* redis


### Actions

#### **Set**
set key to hold the string value
```gherkin
Given cache $resource stores $key with value $value

```

#### **Check**
compares cached content after an action
```gherkin
Given cache $resource stored key $key should look like $value

```

#### **Exists**
check if such key exists in the cache
```gherkin
Given cache $resource has key $key

```

#### **Not Exists**
check if such key doesn't exists in the cache
```gherkin
Given cache $resource hasn't key $key

```



