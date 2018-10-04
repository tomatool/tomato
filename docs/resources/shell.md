# Shell

Initialize resource in `config.yml`:
```yaml
resources:
  - name: # name of the resource
    type: shell
```

## Actions
**Notes:** saved output will be cleared after contains or not contains performed

* Execute - to execute command
```gherkin
Given "$resourceName" execute "$command"
```

* Stdout Contains - to check if stdout contains substring
```gherkin
Given "$resourceName" stdout should contains "$substring"
```

* Stdout Not Contains - to check if stdout contains substring
```gherkin
Given "$resourceName" stdout should not contains "$substring"
```

* Stderr Contains - to check if stderr contains substring
```gherkin
Given "$resourceName" stderr should contains "$substring"
```

* Stderr Not Contains - to check if stderr contains substring
```gherkin
Given "$resourceName" stderr should not contains "$substring"
```
