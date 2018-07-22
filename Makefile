branch = $(shell git symbolic-ref HEAD 2>/dev/null)
version = 0.1.0
revision = $(shell git log -1 --pretty=format:"%H")
build_user = $(USER)
build_date = $(shell date +%FT%T%Z)
pwd = $(shell pwd)

build_dir ?= bin/

pkgs          = ./...
version_pkg= github.com/alileza/gebet/util/version
ldflags := "-X $(version_pkg).Version=$(version) -X $(version_pkg).Branch=$(branch) -X $(version_pkg).Revision=$(revision) -X $(version_pkg).BuildUser=$(build_user) -X $(version_pkg).BuildDate=$(build_date)"


deps:
	@echo " > Installing dependencies"
	@go get -u github.com/golang/dep/cmd/dep
	@dep ensure --vendor-only

build:
	@echo ">> building binaries"
	@go build -ldflags $(ldflags) -o $(build_dir)/gebet main.go
