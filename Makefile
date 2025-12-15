.PHONY: build install run clean

# Default target - install to PATH so `partner` always runs latest
build: install

# Install to $GOPATH/bin (what `partner` command uses)
install:
	go install ./cmd/partner

# Build and run immediately (for quick testing)
run: install
	partner

# Clean build artifacts
clean:
	go clean ./...
