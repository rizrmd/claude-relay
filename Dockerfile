# Build stage
FROM rust:1.82-slim AS builder

# Install required system dependencies
RUN apt-get update && apt-get install -y \
    pkg-config \
    libssl-dev \
    && rm -rf /var/lib/apt/lists/*

# Create app directory
WORKDIR /app

# Copy manifests and source code
COPY Cargo.toml Cargo.lock ./
COPY src ./src

# Build optimized release binary
RUN cargo build --release

# Runtime stage - use debian slim for shell access during auth
FROM debian:bookworm-slim

# Install minimal runtime dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Copy the binary from builder stage
COPY --from=builder /app/target/release/clay /usr/local/bin/clay

# Create app user
RUN useradd -r -m clay
USER clay
WORKDIR /app

# Expose the default port
EXPOSE 3000

# Set default command
CMD ["clay", "--dir", "/app/.clay"]