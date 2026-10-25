ECLIPSE_BIN := eclipse
ECLIPSED_BIN := eclipsed

.PHONY: all web go hub hosted clean

all: web go
hosted: hub eclipsed

web:
	cd web && npm ci && npm run build
	rm -rf internal/webui/dist
	cp -r web/dist internal/webui/dist

go:
	go build -o $(ECLIPSE_BIN) ./cmd/eclipse

hub:
	cd hub && npm ci && npm run build
	rm -rf cmd/eclipsed/dist
	cp -r hub/dist cmd/eclipsed/dist

eclipsed: hub
	go build -o $(ECLIPSED_BIN) ./cmd/eclipsed

clean:
	rm -rf $(ECLIPSE_BIN) $(ECLIPSED_BIN) web/dist internal/webui/dist web/node_modules hub/dist hub/node_modules cmd/eclipsed/dist
