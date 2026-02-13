.PHONY: dev dev-stop build run run-stop docker deploy stop clean

# Local dev: frontend + backend
dev:
	@lsof -ti:9871 | xargs kill -9 2>/dev/null; lsof -ti:9872 | xargs kill -9 2>/dev/null; sleep 1
	@echo "Starting backend :9871 + frontend :9872..."
	@cd server && nohup go run ./cmd/server/ --config etc/config-dev.yaml > /dev/null 2>&1 &
	@cd web && nohup npm run dev > /tmp/smart-daily-frontend.log 2>&1 &
	@sleep 3 && echo "✅ Running — backend :9871 (log: server/logs/), frontend :9872"

# Local dev: stop
dev-stop:
	@lsof -ti:9871 | xargs kill -9 2>/dev/null; lsof -ti:9872 | xargs kill -9 2>/dev/null
	@echo "✅ Stopped"

# Build single binary
build:
	cd web && npm run build
	cp web/mo-logo.png server/cmd/server/dist/
	cd server && CGO_ENABLED=0 go build -o ../bin/smart-daily ./cmd/server/
	@echo "✅ Binary: bin/smart-daily"

# Run binary (production, survives SSH disconnect)
run: build
	@lsof -ti:9871 | xargs kill -9 2>/dev/null; sleep 1
	@nohup ./bin/smart-daily --config server/etc/config-dev.yaml > /dev/null 2>&1 & echo $$! > .pid
	@echo "✅ Running on :9871 (pid: $$(cat .pid))"

# Stop binary
run-stop:
	@if [ -f .pid ]; then kill $$(cat .pid) 2>/dev/null; rm -f .pid; fi
	@lsof -ti:9871 | xargs kill -9 2>/dev/null
	@echo "✅ Stopped"

# Docker
docker:
	docker build -t smart-daily .

deploy:
	docker compose up -d --build
	@echo "✅ Running on :9871"

stop:
	docker compose down

clean:
	rm -rf bin/ server/cmd/server/dist/
