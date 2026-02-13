.PHONY: dev dev-stop build run run-stop docker deploy stop clean

# Local dev: frontend + backend
dev:
	@lsof -ti:9871 | xargs kill -9 2>/dev/null; lsof -ti:9872 | xargs kill -9 2>/dev/null; sleep 1
	@echo "Starting backend :9871 + frontend :9872..."
	@mkdir -p logs
	@nohup go run ./server/cmd/server/ --config server/etc/config-dev.yaml > /dev/null 2>&1 &
	@cd web && nohup npm run dev > ../logs/frontend.log 2>&1 &
	@sleep 3; if lsof -ti:9871 > /dev/null 2>&1; then echo "✅ Backend :9871 OK"; else echo "❌ Backend failed, check logs/backend.log"; fi
	@if lsof -ti:9872 > /dev/null 2>&1; then echo "✅ Frontend :9872 OK"; else echo "❌ Frontend failed, check logs/frontend.log"; fi

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
	@mkdir -p logs
	@set -a; [ -f .env ] && . .env; set +a; nohup ./bin/smart-daily --config server/etc/config-dev.yaml > /dev/null 2>&1 & echo $$! > .pid
	@sleep 2; if lsof -ti:9871 > /dev/null 2>&1; then echo "✅ Running on :9871 (pid: $$(cat .pid))"; else echo "❌ Failed to start, check logs/backend.log"; cat .pid | xargs kill 2>/dev/null; rm -f .pid; exit 1; fi

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
