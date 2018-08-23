project_name = tomato
branch = $(shell git symbolic-ref HEAD 2>/dev/null)
version = 0.1.3
revision = $(shell git log -1 --pretty=format:"%H")
build_user = $(USER)
build_date = $(shell date +%FT%T%Z)
pwd = $(shell pwd)

build_dir ?= bin/

pkgs          = ./...
main_pkg= github.com/alileza/tomato
ldflags := "-X $(main_pkg).Version=$(version) -X $(main_pkg).Branch=$(branch) -X $(main_pkg).Revision=$(revision) -X $(main_pkg).BuildUser=$(build_user) -X $(main_pkg).BuildDate=$(build_date)"


deps:
	@echo " > Installing dependencies"
	@go get -u github.com/golang/dep/cmd/dep
	@dep ensure --vendor-only

build:
	@echo ">> building binaries"
	@go build -ldflags $(ldflags) -o $(build_dir)/tomatool cmd/tomatool/main.go
	@go build -ldflags $(ldflags) -o $(build_dir)/$(project_name) cmd/$(project_name)/main.go

build-all:
	@echo ">> packaging releases"
	@rm -rf dist
	@mkdir dist
	@for os in "linux" "darwin" ; do \
			for arch in "amd64" "386" "arm" "arm64" ; do \
					echo " > building $$os/$$arch" ; \
					GOOS=$$os GOARCH=$$arch go build -ldflags $(ldflags) -o $(build_dir)/$(project_name).$(version).$$os-$$arch cmd/$(project_name)/main.go ; \
			done ; \
	done

test:
	@docker-compose down
	@docker volume ls -q | grep tomato | xargs docker volume rm -f
	@docker-compose up --build --exit-code-from tomato

package-releases:
	@echo ">> packaging releases"
	@rm -rf dist
	@mkdir dist
	@for f in $(shell ls bin) ; do \
			cp bin/$$f tomato ; \
			tar -czvf $$f.tar.gz tomato ; \
			mv $$f.tar.gz dist ; \
			rm -rf tomato ; \
	done
