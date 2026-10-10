BINARY := novo
INSTALL_DIR := $(HOME)/.local/bin
COVER_PROFILE := coverage.out
COVER_MIN := 80

.PHONY: build install uninstall fmt test cover cover-html clean

build:
	go build -o $(BINARY) .

install: test build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	chmod +x $(INSTALL_DIR)/$(BINARY)
	@echo "installed: $(INSTALL_DIR)/$(BINARY)"

uninstall:
	rm -f $(INSTALL_DIR)/$(BINARY)

fmt:
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:"; gofmt -l .; exit 1)
	go vet ./...

# test runs the unit tests with the race detector. Use `cover` for a
# coverage report with an enforced minimum.
test: fmt
	go test ./... -race

# cover runs the tests with coverage instrumentation and fails the build
# if total statement coverage drops below COVER_MIN (industry-standard
# gate; 80% is a common baseline for Go projects).
cover: fmt
	go test ./... -race -covermode=atomic -coverprofile=$(COVER_PROFILE)
	@go tool cover -func=$(COVER_PROFILE) | tail -1
	@pct=$$(go tool cover -func=$(COVER_PROFILE) | tail -1 | grep -oE '[0-9]+\.[0-9]+'); \
	LC_ALL=C awk -v p="$$pct" -v min=$(COVER_MIN) 'BEGIN { \
		if (p+0 < min+0) { printf "FAIL: coverage %.1f%% is below the %d%% minimum\n", p, min; exit 1 } \
		else { printf "OK: coverage %.1f%% >= %d%% minimum\n", p, min } \
	}'

cover-html: cover
	go tool cover -html=$(COVER_PROFILE) -o coverage.html
	@echo "report: coverage.html"

clean:
	rm -f $(BINARY) $(COVER_PROFILE) coverage.html
