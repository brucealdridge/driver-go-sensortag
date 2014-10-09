GOPATH=$(shell pwd)/.gopath

debug:
	scripts/build.sh

clean:
	rm -f bin/driver-go-flowerpower || true
	rm -rf .gopath || true

test:
	cd .gopath/src/github.com/ninjasphere/driver-go-flowerpower && go get -t ./...
	cd .gopath/src/github.com/ninjasphere/driver-go-flowerpower && go test ./...

vet:
	go vet ./...

.PHONY: debug clean test vet
