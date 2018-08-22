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

database driver to interact with sql database

### resource parameters

1. **driver** *(string)*

   sql driver (postgres or mysql)

1. **datasource** *(string)*

   sql database source name (e.g: postgres://user:pass@host:port/dbname?sslmode=disable)


## actions

### **set**

   set table content (it will truncate table before set the value)

   **expressions**
	 
   - set $resource table $table list of content $content
	 

   **parameters**
	 
   - table *(string)*

     table name
	 
   - content *(table)*

     table content
	 


### **check**

   compare table content

   **expressions**
	 
   - $resource table $table should look like $content
	 

   **parameters**
	 
   - table *(string)*

     table name
	 
   - content *(table)*

     table content
	 


### **empty**

   truncate table and reset auto increment

   **expressions**
	 
   - set $resource table $table to empty
	 

   **parameters**
	 
   - table *(string)*

     table name
	 

---


# http
## client

http client that can be use for sending http request, and compare the response.

### resource parameters

1. **base_url** *(string)*

   base url for the http client, it will automatically appended as a base target url.

1. **timeout** *(duration)*

   timeout for the request roundtrip.


## actions

### **send**

   send a normal http request without request body

   **expressions**
	 
   - $resource send request to $target
	 

   **parameters**
	 
   - target *(string)*

     http target endpoint, it has to be space separated between request method and URL
	 


### **send_body**

   send a normal http request with request body

   **expressions**
	 
   - $resource send request to $target with body $body
	 
   - $resource send request to $target with payload $body
	 

   **parameters**
	 
   - target *(string)*

     http target endpoint, it has to be space separated between request method and URL
	 
   - body *(json)*

     http request body payload
	 


### **response_code**

   check response code

   **expressions**
	 
   - $resource response code should be $code
	 

   **parameters**
	 
   - code *(number)*

     http response code
	 


### **response_body**

   check response body

   **expressions**
	 
   - $resource response body should be $body
	 

   **parameters**
	 
   - body *(json)*

     http response body
	 

---


## server

http server that can be use for having a mock external http server.

### resource parameters

1. **port** *(number)*

   http server port to expose


## actions

### **response**

   set a response for any request that come to the http/server

   **expressions**
	 
   - set $resource response code to $code and response body $body
	 

   **parameters**
	 
   - code *(number)*

     server response code
	 
   - body *(json)*

     server response body
	 


### **response_path**

   set a response for a given path for the http/server

   **expressions**
	 
   - set $resource with path $path response code to $code and response body $body
	 

   **parameters**
	 
   - path *(string)*

     server endpoint path
	 
   - code *(number)*

     server response code
	 
   - body *(json)*

     server response body
	 

---


# queue
## queue

message queue resource to interact with queue

### resource parameters

1. **driver** *(string)*

   queue driver (rabbitmq)

1. **datasource** *(string)*

   queue source name (e.g: amqp://user:pass@host:port/)


## actions

### **publish**

   publish a message to message queue

   **expressions**
	 
   - publish message to $resource target $target with payload $payload
	 

   **parameters**
	 
   - target *(string)*

     target would be depending on the driver, on rabbitmq driver target consist of `[exchange]:[routing-key]`
	 
   - payload *(json)*

     queue message payload
	 


### **listen**

   listen to message queue message, it required to do before the action,
if you need to compare message that got published from the application


   **expressions**
	 
   - listen message from $resource target $target
	 

   **parameters**
	 
   - target *(string)*

     target would be depending on the driver, on rabbitmq driver target consist of `[exchange]:[routing-key]`
	 


### **count**

   count message from a given target, it required to listen to target before compare it. Otherwise, it would compare to an empty queue.

   **expressions**
	 
   - message from $resource target $target count should be $count
	 

   **parameters**
	 
   - target *(string)*

     target would be depending on the driver, on rabbitmq driver target consist of `[exchange]:[routing-key]`
	 
   - count *(number)*

     queue message count
	 


### **compare**

   compare message payload, it required to listen to target before compare it. Otherwise, it would compare to an empty queue.

   **expressions**
	 
   - message from $resource target $target should look like $payload
	 

   **parameters**
	 
   - target *(string)*

     target would be depending on the driver, on rabbitmq driver target consist of `[exchange]:[routing-key]`
	 
   - payload *(json)*

     queue message payload
	 

---

