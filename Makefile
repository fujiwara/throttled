.PHONY: clean test

throttled: go.* *.go
	go build -o $@ ./cmd/throttled

clean:
	rm -rf throttled dist/

test:
	go test -v ./...

install:
	go install github.com/fujiwara/throttled/cmd/throttled

dist:
	goreleaser build --snapshot --clean
