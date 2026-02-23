FROM caddy:builder AS builder

RUN xcaddy build \
    --with github.com/mholt/caddy-ratelimit

# Step 2: Assemble the final hardened image
FROM caddy:2.7-alpine

# Copy the custom binary from the builder
COPY --from=builder /usr/bin/caddy /usr/bin/caddy