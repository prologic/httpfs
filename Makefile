.PHONY: dev install profile bench test clean

all: dev

dev:
	@go build -o httpfsd ./cmd/httpfsd
	@go build -o httpfsmount ./cmd/httpfsmount

install:
	@go install ./...

profile:
	@go test -cpuprofile cpu.prof -memprofile mem.prof -v -bench . ./...

bench:
	@go test -bench . ./...

test:
	@go test \
		-race \
		-cover \
		-coverprofile=coverage.txt \
		-covermode=atomic \
		-coverpkg=./... \
		./...

clean:
	@rm -rf httpfsd httpfsmount coverage.txt
