# ipamthing

Simple, low-dependency web-based IPAM tool. Go + SQLite, single static binary.

## Features

- Subnet management (CIDR, name, scan interval, enable/disable)
- Per-subnet host discovery (ICMP + neighbor table + TCP fallback)
- IPv4 (cap /16) and IPv6 (cap /120) subnets
- Reverse DNS with per-host hostname override
- Per-host comments, free-form tags, scan history
- Home dashboard with totals, per-subnet summary, recent activity
- Site-wide events feed with CSV export
- CSV export per subnet
- Client-side filter on the hosts table
- Bulk delete / set-comment / set-tags on selected hosts
- Manual scan and continuous background monitoring
- Basic username/password login

## Flags

- `-addr` — HTTP listen address (default `:8080`)
- `-db` — SQLite database path (default `ipamthing.db`)

## Build

Requires Go 1.26+. `modernc.org/sqlite` is pure-Go (no cgo), so cross-compiles
cleanly to any target.

```
go mod tidy
go build -o ipamthing .
```

For a stripped static Linux amd64 binary (suitable for scp to a server):

```
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o ipamthing-linux-amd64 .
```

## Run locally

```
./ipamthing -addr :8080 -db ipamthing.db
```

Open <http://localhost:8080>. First visit prompts to create an admin user.

## ICMP privileges

ICMP scanning needs one of:

- `setcap cap_net_raw+ep ./ipamthing` (per-binary)
- Running under systemd with `AmbientCapabilities=CAP_NET_RAW` (see below)
- The running user's GID inside `net.ipv4.ping_group_range`:
  ```
  sudo sysctl -w net.ipv4.ping_group_range="0 2147483647"
  ```

Without ICMP, scanning falls back to neighbor-table reads + TCP connect probes
on common ports; a notice appears on the subnet page.

### Neighbor/MAC discovery

Uses `ip neigh show` when the `iproute2` `ip` tool is on PATH (covers both
IPv4 and IPv6), otherwise falls back to `/proc/net/arp` (IPv4 only).

## Deploy to a Linux server

The intended deployment is a single static binary + systemd unit + dedicated
service user. No runtime dependencies beyond the kernel.

### 1. Cross-compile on your dev box

```
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o ipamthing-linux-amd64 .
scp -P <port> ipamthing-linux-amd64 <user>@<host>:/tmp/
```

(Use `GOARCH=arm64` for a Raspberry Pi or similar.)

### 2. On the server (once, as root)

```
sudo useradd --system --home-dir /var/lib/ipamthing --create-home \
             --shell /usr/sbin/nologin ipamthing
sudo install -o root -g root -m 0755 /tmp/ipamthing-linux-amd64 /usr/local/bin/ipamthing
```

### 3. systemd unit

Write to `/etc/systemd/system/ipamthing.service`:

```ini
[Unit]
Description=ipamthing
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/ipamthing -addr :8080 -db /var/lib/ipamthing/ipamthing.db
User=ipamthing
Group=ipamthing
AmbientCapabilities=CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_RAW
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/ipamthing
PrivateTmp=true
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

Then:

```
sudo systemctl daemon-reload
sudo systemctl enable --now ipamthing
sudo systemctl status ipamthing
```

`AmbientCapabilities=CAP_NET_RAW` gives the unprivileged `ipamthing` user
just enough to open raw ICMP sockets; `NoNewPrivileges` + `ProtectSystem`
+ `ProtectHome` + `ReadWritePaths` keep the blast radius tight.

### 4. Upgrading

```
# on dev box
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o ipamthing-linux-amd64 .
scp -P <port> ipamthing-linux-amd64 <user>@<host>:/tmp/

# on server
sudo install -o root -g root -m 0755 /tmp/ipamthing-linux-amd64 /usr/local/bin/ipamthing
sudo systemctl restart ipamthing
```

The SQLite schema auto-migrates on startup (idempotent `ALTER TABLE` guards).
