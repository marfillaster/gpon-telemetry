# GPON telemetry dashboard for RouterOS containers.
#
# Build:
#   CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath \
#     -ldflags "-s -w" -o gponserve ./cmd/gponserve
#   CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath \
#     -ldflags "-s -w" -o gpontelemetry ./cmd/gpontelemetry
#   docker build --platform linux/arm64 -t gpon-telemetry:latest .
#   docker save gpon-telemetry:latest -o gpon-telemetry.tar
#
# RouterOS runs /gpontelemetry sample via /container/shell ... as-value,
# logs the captured GPONRAW line through its own disk logger, then runs
# /gpontelemetry all to refresh web-visible rollup files.

FROM alpine:3.22

COPY gponserve /gponserve
COPY gpontelemetry /gpontelemetry
COPY web/index.html /opt/gpontelemetry/www/index.html

ENV GPON_ADDR=":3000" \
    GPON_STATIC_ROOT="/opt/gpontelemetry/www" \
    GPON_LOG_ROOT="/var/lib/gpontelemetry" \
    GPON_HOST="192.168.1.1" \
    GPON_STICK_URL="http://192.168.1.1" \
    GPON_USER="admin" \
    GPON_PASS="admin"

ENTRYPOINT ["/gponserve"]
