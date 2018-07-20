branch = $(shell git symbolic-ref HEAD 2>/dev/null)
version = 0.1.0
revision = $(shell git log -1 --pretty=format:"%H")
build_user = $(USER)
build_date = $(shell date +%FT%T%Z)
pwd = $(shell pwd)

build_dir ?= bin/

pkgs          = ./...
ldflags := "-X main.Version=$(version) -X main.Branch=$(branch) -X main.Revision=$(revision) -X main.BuildUser=$(build_user) -X main.BuildDate=$(build_date)"


deps:
	@echo " > Installing dependencies"
	@go get -u github.com/golang/dep/cmd/dep
	@dep ensure --vendor-only

build:
	@echo ">> building binaries"
	@go build -ldflags $(ldflags) -o $(build_dir)/gebet cmd/gebet/main.go
