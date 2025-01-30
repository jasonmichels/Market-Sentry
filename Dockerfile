# ----------------------------------
#          Build Stage
# ----------------------------------
FROM golang:1.23 AS build-stage

WORKDIR /app

# 1) Copy only go.mod & go.sum first for caching
COPY go.mod go.sum ./
RUN go mod download

# 2) Copy the rest of the code
COPY . .

# 3) Build the Go app
#    - Adjust path if your main is under cmd/server
#    - -ldflags='-s -w' strips debug info for a smaller binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags='-s -w' -o marketsentry ./cmd/server

# ----------------------------------
#          Final Stage
# ----------------------------------
FROM gcr.io/distroless/base

# Expose the port your app listens on (assuming 8080)
EXPOSE 8080

# Copy the compiled binary from the build stage
COPY --from=build-stage /app/marketsentry /marketsentry

# If you have a folder with static files/templates, copy that too:
# (Adjust if you use "web/" or "public/")
COPY --from=build-stage /app/web /web

# Run the binary
ENTRYPOINT ["/marketsentry"]
