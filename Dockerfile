# ==============================================================================
# Stage 1: Builder
# ==============================================================================
FROM golang:1.22-alpine AS builder

# Install build dependencies for CGO and ONNX Runtime
RUN apk add --no-cache \
    build-base \
    curl \
    tar \
    ca-certificates

# Download and install ONNX Runtime
ARG ONNX_VERSION=1.16.0
RUN curl -L https://github.com/microsoft/onnxruntime/releases/download/v${ONNX_VERSION}/onnxruntime-linux-x64-${ONNX_VERSION}.tgz \
    -o /tmp/onnxruntime.tgz && \
    mkdir -p /opt/onnxruntime && \
    tar -xzf /tmp/onnxruntime.tgz -C /opt/onnxruntime --strip-components=1 && \
    rm /tmp/onnxruntime.tgz

# Set environment for ONNX Runtime
ENV CGO_ENABLED=1
ENV ONNXRUNTIME_LIB_PATH=/opt/onnxruntime/lib
ENV LD_LIBRARY_PATH=/opt/onnxruntime/lib:$LD_LIBRARY_PATH
ENV LIBRARY_PATH=/opt/onnxruntime/lib:$LIBRARY_PATH
ENV C_INCLUDE_PATH=/opt/onnxruntime/include:$C_INCLUDE_PATH

WORKDIR /app

# Copy go.mod and go.sum first for dependency caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN go build -ldflags="-s -w" -o server ./cmd/server/main.go

# ==============================================================================
# Stage 2: Runtime
# ==============================================================================
FROM gcr.io/distroless/base-debian12

WORKDIR /app

# Copy the binary
COPY --from=builder /app/server .

# Copy ONNX Runtime shared libraries
COPY --from=builder /opt/onnxruntime/lib/libonnxruntime*.so* /usr/local/lib/

# Set library path
ENV LD_LIBRARY_PATH=/usr/local/lib

# Expose gRPC and metrics ports
EXPOSE 50051
EXPOSE 9100

# Run the server
ENTRYPOINT ["./server"]
CMD ["-port", "50051", "-metrics", "9100"]
