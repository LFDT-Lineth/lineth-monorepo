.PHONY: test-db test-db-migrate clean

# Start test DB and apply migrations (for testing)
test-db:
	docker compose -f docker-compose.test.yml up -d
	@echo "Waiting for database to be ready..."
	@sleep 2
	DATABASE_URL=postgresql://testuser:testpass@localhost:5433/lido_governance_monitor_test pnpm db:deploy

# Generate a new migration (development only - Prisma 7 requires explicit generate before migrate)
test-db-migrate:
	docker compose -f docker-compose.test.yml up -d
	@echo "Waiting for database to be ready..."
	@sleep 2
	DATABASE_URL=postgresql://testuser:testpass@localhost:5433/lido_governance_monitor_test pnpm db:generate
	DATABASE_URL=postgresql://testuser:testpass@localhost:5433/lido_governance_monitor_test pnpm db:migrate

clean:
	docker compose -f docker-compose.test.yml down -v
