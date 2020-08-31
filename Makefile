project_name = tomato
branch = $(shell git symbolic-ref HEAD 2>/dev/null)
version = 1.2.0
revision = $(shell git log -1 --pretty=format:"%H")
build_user = $(USER)
build_date = $(shell date +%FT%T%Z)
pwd = $(shell pwd)

build_dir ?= bin/

pkgs          = ./...
version_pkg= github.com/tomatool/tomato/version
ldflags := "-X $(version_pkg).Version=$(version) -X $(version_pkg).Branch=$(branch) -X $(version_pkg).Revision=$(revision) -X $(version_pkg).BuildUser=$(build_user) -X $(version_pkg).BuildDate=$(build_date)"


deps:
	@echo " > Installing dependencies"
	@go get -u github.com/golang/dep/cmd/dep
	@dep ensure --vendor-only

build-ui:
	@echo ">> building ui"
	@cd ui && npm run build
	@pkger

build:
	@echo ">> building binaries"
	@go build -mod vendor -ldflags $(ldflags) -o $(build_dir)/tomatool cmd/tomatool/main.go
	@go build -mod vendor -ldflags $(ldflags) -o $(build_dir)/$(project_name) .

build-test:
	@echo ">> building binaries"
	@go test -coverpkg="./..." github.com/tomatool/$(project_name) -c -tags testmain -o $(build_dir)/$(project_name).test
	@go build -ldflags $(ldflags) -o $(build_dir)/tomatool cmd/tomatool/main.go

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

check:
	@go run cmd/tomatool/main.go generate docs -t markdown -o /tmp/docs
	@diff -q /tmp/docs docs/resources.md || (echo "$$? inconsistent dictionary with documentation, please run 'make gen'"; exit 1)
	@go run cmd/tomatool/main.go generate handler -o /tmp/handler
	@git diff --exit-code || (echo "$$? inconsistent dictionary with handler, please run 'make gen'"; exit 1)

gen:
	@echo ">> generating markdown documentation"
	@go run cmd/tomatool/main.go generate docs -t markdown
	@echo ">> generating handler"
	@go run cmd/tomatool/main.go generate handler
	@echo ">> generating mocks"
	@mockgen -destination=handler/queue/mocks/Resource.go -package=mocks -source=handler/queue/queue.go

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
