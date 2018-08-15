# Resources

Resources are objects that are going to be used for step evaluations in the cucumber scenario. They are listed under the resources key in the pipeline configuration.

Supported resources:

* db
   - [sql](#db/sql)
  
* http
   - [client](#http/client)
   - [server](#http/server)
  
* queue
   - [queue](#queue/queue)
  
---

# db
## sql
**Parameters**
  
    - driver:   
    - datasource:   
**Available functions**

  1. **set**
    set table content    
      set $resource table $table list of content $content
        
  1. **check**
    compare table content    
      $resource table $table should look like $content
        
  1. **empty**
    truncate table    
      set $resource table $table to empty
        
# http
## client
**Parameters**
  
    - base_url: base url for the http client, it will automatically appended as a base target url.  
    - timeout: timeout for the request roundtrip.  
**Available functions**

  1. **send**
    send a normal http request without request body    
      $resource send request to $target
        
  1. **send_body**
    send a normal http request without request body    
      $resource send request to $target with body $body
        
  1. **response_code**
    check resposne code    
      $resource response code should be $code
        
  1. **response_body**
    check resposne body    
      $resource response body should be $body
        
## server
**Parameters**
  
    - port: http server port to expose  
**Available functions**

  1. **response**
    set a response for any request that come to the http/server    
      set $resource response code to $code and response body $body
        
  1. **response_path**
    set a response for a given path for the http/server    
      set $resource with path $path response code to $code and response body $body
        
# queue
## queue
**Parameters**
  
    - driver:   
    - datasource:   
**Available functions**

  1. **publish**
    publish message    
      publish message to $resource target $target with payload $payload
        
  1. **listen**
    listen to message    
      listen message from $resource target $target
        
  1. **count**
    count message on taarget    
      message from $resource target $target count should be $count
        
  1. **compare**
    compare message payload    
      message from $resource target $target should look like $payload
        