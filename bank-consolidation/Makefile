BANK_BINARY=bank-consolidationApp

## up: starts all containers in the background without forcing build
up:
	@echo "Starting Docker images..."
	docker compose up -d
	@echo "Docker images started!"

up_build: build-bank-consolidation
	@echo "Stopping docker compose..."
	docker compose down
	@echo "Up Docker compose"
	docker compose up  --build -d


down: 
	@echo "Stopping docker compose..."
	docker compose down
	@echo "Done!"


build-bank-consolidation: frontend-build
	@echo "Building bank-consolidation image..."
	cd ../bank-consolidation && env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ${BANK_BINARY} .
	@echo "Done!"

frontend-build:
	@echo "Building frontend..."
	cd frontend && npm install && npm run build
	@echo "Frontend built!"

clean-db:
	@printf "Warning: This will delete all data in bank.db. Are you sure? [y/N] "; \
	read ans; \
	if [ "$$ans" != "y" ]; then \
		echo "Aborted."; \
		exit 1; \
	fi
	@echo "Backing up database..."
	cp data/bank.db data/bank.db.bak
	@echo "Cleaning up database..."
	sqlite3 data/bank.db < data/cleanup_data.sql
	@echo "Database cleaned up and vacuumed."
