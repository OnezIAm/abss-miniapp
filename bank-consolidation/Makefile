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


build-bank-consolidation:
	@echo "Building bank-consolidation image..."
	cd ../bank-consolidation && env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ${BANK_BINARY} .
	@echo "Done!"
