# Database/SQL

Initialize resource in `config.yml`:
```
- name: # name of the resource
  type: database/sql
  ready_check: true
  params:
    driver: # driver
    datasource: # data source name of the database
```

* MySQL
```
params:
  driver: mysql
  datasource: root:potato@tcp(127.0.0.1:3306)/tomato
```

* Postgres
```
params:
  driver: postgres
  datasource: postgres://tomato:potato@localhost:5432/tomato?sslmode=disable
```

## Actions
* Compare - to compare value on database with a given table definition
```gherkin
Given "$resourceName" table "$tableName" should look like
    | $columnName1 | $columnName2 |
    | $value1      | $value2      |
```

* Insert - to insert value to database
```gherkin
Given set "$resourceName" table "$tableName" list of content$
    | $columnName1 | $columnName2 |
    | $value1      | $value2      |
```
