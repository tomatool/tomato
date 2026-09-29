# AWS

Steps for the AWS identity (IRSA) tomato gives the application: which roles it assumed at tomato's STS

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Identity

| Step | Description |
|------|-------------|
| `"{resource}" role "arn:aws:iam::000000000000:role/my-service" was assumed` | Asserts the application assumed the role at tomato's STS during the run |
| `"{resource}" role "arn:aws:iam::000000000000:role/my-service" was not assumed` | Asserts the application never assumed the role at tomato's STS during the run |


