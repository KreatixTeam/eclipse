ECLIPSE_BIN := eclipse

.PHONY: all web go clean

all: web go

web:
	cd web && npm ci && npm run build
	rm -rf internal/webui/dist
	cp -r web/dist internal/webui/dist

go:
	go build -o $(ECLIPSE_BIN) ./cmd/eclipse

clean:
	rm -rf $(ECLIPSE_BIN) web/dist internal/webui/dist web/node_modules
