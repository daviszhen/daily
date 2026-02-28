.PHONY: dev dev-stop run run-stop build clean

# ============ 本地开发（热更新）============
# 前端 :9872 + 后端 :9871，改代码自动刷新
dev:
	@lsof -ti:9871 | xargs kill -9 2>/dev/null; lsof -ti:9872 | xargs kill -9 2>/dev/null; sleep 1
	@mkdir -p logs
	@cd server && nohup go run ./cmd/server/ --config etc/config-dev.yaml > /dev/null 2>&1 &
	@cd web && nohup npm run dev > ../logs/frontend.log 2>&1 &
	@sleep 3; if lsof -ti:9871 > /dev/null 2>&1; then echo "✅ Backend :9871"; else echo "⏳ Waiting..."; sleep 5; echo "✅ Backend :9871"; fi
	@if lsof -ti:9872 > /dev/null 2>&1; then echo "✅ Frontend :9872"; else echo "❌ Frontend failed"; fi

dev-stop:
	@lsof -ti:9871 | xargs kill -9 2>/dev/null; lsof -ti:9872 | xargs kill -9 2>/dev/null
	@echo "✅ Stopped"

# ============ 部署（单二进制）============
# 编译前端+后端 → 单 binary 跑在 :9871
run: build
	@lsof -ti:9871 | xargs kill -9 2>/dev/null; sleep 1
	@mkdir -p logs
	@set -a; [ -f .env ] && . .env; set +a; nohup ./bin/smart-daily --config server/etc/config-dev.yaml > /dev/null 2>&1 & echo $$! > .pid
	@sleep 2; if lsof -ti:9871 > /dev/null 2>&1; then echo "✅ Running on :9871 (pid: $$(cat .pid))"; else echo "⏳ Waiting..."; sleep 5; echo "✅ Running on :9871 (pid: $$(cat .pid))"; fi

run-stop:
	@if [ -f .pid ]; then kill $$(cat .pid) 2>/dev/null; rm -f .pid; fi
	@lsof -ti:9871 | xargs kill -9 2>/dev/null
	@echo "✅ Stopped"

# ============ 辅助 ============
build:
	cd web && npm run build
	cp web/mo-logo.png server/cmd/server/dist/
	cd server && CGO_ENABLED=0 go build -o ../bin/smart-daily ./cmd/server/
	@echo "✅ Binary: bin/smart-daily"

clean:
	rm -rf bin/ server/cmd/server/dist/ exports/
