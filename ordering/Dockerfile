# base image (golang:1.26) from which the base image will
# be created
FROM golang:1.26 AS builder

# creates a new directory (/workspace) insde the container 
# and set's it as the current working directory
WORKDIR /workspace

# copy the foundation and epot local folders to the container's
# /workspace/foundation and /workspace/depot folders repectively
COPY foundation/ ./foundation
COPY depot-baskets/depot ./depot-baskets/depot

# copy the depot local folder to the container's 
# /workspace/ordering folder
COPY ordering/ ./ordering

# Set the working directory to depot
WORKDIR /workspace/ordering

# download the dependencies listed in go.mod
RUN go mod download

# Disable the C library for go. This will force the go compiler to 
# build without C dependencies. Runs the builder to build the binary 
# from the container's ./cmd/service package and place it in
# (/workspace/bin/depot)
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/ordering ./cmd/service

# === 2ND STAGE ===
# Get a new alpine image
FROM alpine:3.20

# Run the apk package manager and add Certicate Authority certifcates
# for signing the server TLS certifcates
RUN apk add --no-cache ca-certificates

# set the working directory to /app
WORKDIR /app

# copyt the /bin/depot binary to /app
COPY --from=builder /bin/ordering ./ordering

# all services operating in containers will be bound to port 8080
EXPOSE 8080

# run the depot binary
CMD ["./ordering"]
