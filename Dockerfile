FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod ./
RUN go mod download

# Copy source code
COPY main.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o controller main.go

# Final stage
FROM alpine:latest

# Install required tools: git, helm, kubectl, openssl
RUN apk add --no-cache \
    ca-certificates \
    git \
    curl \
    bash \
    openssl

# Install kubectl
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    chmod +x kubectl && \
    mv kubectl /usr/local/bin/

# Install helm
RUN curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

WORKDIR /root/

# Clone code-server repository at build time
RUN git clone --depth 1 https://github.com/coder/code-server.git /opt/code-server && \
    echo "Code-server repository cloned successfully" && \
    ls -la /opt/code-server/ci/helm-chart

# Copy the binary from builder
COPY --from=builder /app/controller .

EXPOSE 8081

CMD ["./controller"]