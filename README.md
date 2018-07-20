# gebet

Behavior Driven Development tools, built on top of (https://github.com/DATA-DOG/godog)

## example
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
