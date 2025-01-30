# Makefile for Market Sentry
# Assumes:
#  - Your main is in cmd/server/main.go
#  - You have a "web/" folder for static assets
#  - Dockerfile (production) and Dockerfile.dev exist

APP_NAME = marketsentry
IMAGE_NAME = jasonmichels/marketsentry

# Build the Go binary locally
build:
	CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o $(APP_NAME) ./cmd/server

# Build & tag the Docker image for production using Dockerfile
build-docker: build
	docker build -t $(IMAGE_NAME):latest .

# Build & tag the dev Docker image using Dockerfile.dev
build-docker-dev: build
	docker build -t $(IMAGE_NAME):dev -f Dockerfile.dev .

# Run tests
test:
	go test -v ./... -bench . -cover

# Spin up via docker-compose (assuming you have a docker-compose.yml)
run:
	docker compose up -d --build

# Example usage:
#   make build             # compile the binary
#   make build-docker      # build production image
#   make build-docker-dev  # build dev image
#   make test              # run tests
#   make run               # bring up via docker-compose
