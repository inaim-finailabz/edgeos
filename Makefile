BINARIES := agent router cli dashboard mcp
PLATFORMS := linux/arm64 linux/amd64 darwin/arm64
DIST := dist

.PHONY: build cross fmt vet test check clean $(BINARIES)

build: $(BINARIES)

$(BINARIES):
	go build -o $(DIST)/$@ ./$@

cross:
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		for bin in $(BINARIES); do \
			echo "building $$bin $$os/$$arch"; \
			GOOS=$$os GOARCH=$$arch go build -o $(DIST)/$$os-$$arch/$$bin ./$$bin || exit 1; \
		done; \
	done

fmt:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "$$out"; exit 1; fi

vet:
	go vet ./...

test:
	go test ./...

check: fmt vet test

clean:
	rm -rf $(DIST)
