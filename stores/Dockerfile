# base image (golang:1.25) from which the base image will
# be created
FROM golang:1.25 AS builder

# creates a new directory (/workspace) insde the container 
# and set's it as the current working directory
WORKDIR /workspace

# copy the foundation local folder to the container's
# /workspace/foundation folder
COPY foundation/ ./foundation

# copy the stores local folder to the container's 
# /workspace/stores folder
COPY stores/ ./stores

# redefine the current working directory as /workspace/stores
WORKDIR /workspace/stores

# download the dependencies listed in go.mod
RUN go mod download

# Disable the C library for go. This will force the go compiler to 
# build without C dependencies. Runs the builder to build the binary 
# from the container's ./cmd/service package and place it in
# (/workspace/stores/bin/stores)
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/stores ./cmd/service

# === 2ND STAGE ===
# Get a new alpine image
FROM alpine:3.20

# Run the apk package manager and add Certicate Authority certifcates
# for signing the server TLS certifcates
RUN apk add --no-cache ca-certificates

# set/create the /app directory and set it as the CWD in the apline image 
WORKDIR /app

# copy the compiled binary from the builder image to the apline image 
# place it inside /app/
COPY --from=builder /bin/stores ./stores

# Expose the container's port 8080 for connectivity 
EXPOSE 8080

# run the ./stores binary command when the container starts 
CMD ["./stores"]
