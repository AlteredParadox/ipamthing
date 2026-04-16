# syntax=docker/dockerfile:1.7

# --- build stage ------------------------------------------------------------
FROM golang:1.26-alpine AS build
WORKDIR /src

# Dependencies first so subsequent builds can cache them.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/ipamthing .

# --- runtime stage ----------------------------------------------------------
# Alpine gives us iproute2 for "ip neigh show" (covers IPv6 neighbor lookups;
# without it we fall back to /proc/net/arp and only see IPv4 MACs).
FROM alpine:3.20

RUN apk add --no-cache iproute2 \
 && addgroup -S ipamthing \
 && adduser -S -G ipamthing -H -h /var/lib/ipamthing ipamthing \
 && mkdir -p /var/lib/ipamthing \
 && chown ipamthing:ipamthing /var/lib/ipamthing

COPY --from=build /out/ipamthing /usr/local/bin/ipamthing

USER ipamthing
EXPOSE 8080
VOLUME ["/var/lib/ipamthing"]

ENTRYPOINT ["/usr/local/bin/ipamthing"]
CMD ["-addr=:8080", "-db=/var/lib/ipamthing/ipamthing.db"]
