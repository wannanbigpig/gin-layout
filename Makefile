.PHONY: dev-up dev-down dev-logs dev-init-bucket dev-service dev-worker

dev-up:
	docker-compose -f docker-compose.dev.yml up -d
	@echo "等待服务启动..."
	@sleep 5
	@echo "MySQL: localhost:3306"
	@echo "Redis: localhost:6379"
	@echo "MinIO API: localhost:9000"
	@echo "MinIO Console: localhost:9001 (minioadmin / minioadmin123)"

dev-down:
	docker-compose -f docker-compose.dev.yml down

dev-logs:
	docker-compose -f docker-compose.dev.yml logs -f

dev-init-bucket:
	@echo "创建 MinIO bucket..."
	docker exec go-layout-minio mc alias set local http://localhost:9000 minioadmin minioadmin123
	docker exec go-layout-minio mc mb local/gin-layout --ignore-existing
	docker exec go-layout-minio mc anonymous set download local/gin-layout/public
	@echo "Bucket 创建完成"

dev-service:
	air -c .air.toml

dev-worker:
	air -c .air.worker.toml
