GOHOSTOS:=$(shell go env GOHOSTOS)
GOPATH:=$(shell go env GOPATH)
VERSION=$(shell git describe --tags --always)

#ifeq ($(GOHOSTOS), windows)
#	#the `find.exe` is different from `find` in bash/shell.
#	#to see https://docs.microsoft.com/en-us/windows-server/administration/windows-commands/find.
#	#changed to use git-bash.exe to run find cli or other cli friendly, caused of every developer has a Git.
#	#Git_Bash= $(subst cmd\,bin\bash.exe,$(dir $(shell where git)))
#	Git_Bash=$(subst \,/,$(subst cmd\,bin\bash.exe,$(dir $(shell where git))))
#	INTERNAL_PROTO_FILES=$(shell $(Git_Bash) -c "find internal -name *.proto")
#	API_PROTO_FILES=$(shell $(Git_Bash) -c "find api -name *.proto")
#else
	INTERNAL_PROTO_FILES=$(shell find internal -name *.proto)
	API_PROTO_FILES=$(shell find api -name *.proto)
#endif

.PHONY: init
# init env
init:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/go-kratos/kratos/cmd/kratos/v2@latest
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
	go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	go install github.com/envoyproxy/protoc-gen-validate@latest
	go install github.com/google/wire/cmd/wire@latest

.PHONY: config
# generate internal proto
config:
	protoc --proto_path=./internal \
	       --proto_path=./third_party \
 	       --go_out=paths=source_relative:./internal \
 	       --validate_out=lang=go,paths=source_relative:./internal \
	       $(INTERNAL_PROTO_FILES)

.PHONY: api
# generate api proto
api:
	protoc --proto_path=./api \
	       --proto_path=./third_party \
 	       --go_out=paths=source_relative:./api \
 	       --go-http_out=paths=source_relative:./api \
 	       --go-grpc_out=paths=source_relative:./api \
 	       --validate_out=paths=source_relative,lang=go:./api \
	       --openapi_out=fq_schema_naming=true,default_response=false:. \
	       $(API_PROTO_FILES)

.PHONY: build
# build for linux amd64
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X main.Version=$(VERSION)" -o ./stress ./cmd/server

.PHONY: wire
# wire
wire:
	cd cmd/server && wire

.PHONY: generate
# generate
generate:
	go generate ./...
	go mod tidy

.PHONY: all
# generate all
all:  api config wire generate
#	make generate

.PHONY: scp
# deploy to remote server
scp: build
	git pull
	bash scripts/deploy.sh ./stress
	rm -f ./stress

.PHONY: task
# create stress task
task:
	bash scripts/create_task.sh

.PHONY: game
# register game. eg: make game id=999999 name=大话西游
game:
	bash ./scripts/gen-game/gen-game.sh "$(id)" "$(name)"

# show help
help:
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
