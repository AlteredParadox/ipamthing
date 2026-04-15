# ipamthing

Simple, low-dependency web-based IPAM tool. Go + SQLite, single binary.

## Features

- Subnet management (CIDR, name, scan interval, enable/disable)
- Per-subnet host discovery (ICMP + neighbor table + TCP fallback)
- IPv4 (cap /16) and IPv6 (cap /120) subnets
- Reverse DNS with per-host hostname override
- Per-host comments and scan history
- Home dashboard with totals + per-subnet summary + recent activity
- Site-wide recent-activity feed (with CSV export)
- CSV export per subnet
- Free-form host tags (space-separated) with client-side filter
- Bulk delete / set-comment / set-tags on selected hosts
- Manual scan and continuous background monitoring
- Basic username/password login

### Neighbor/MAC discovery

Uses `ip neigh show` if available (both IPv4 and IPv6), falling back to
`/proc/net/arp` (IPv4 only). IPv6 MACs are only populated when `ip` is on PATH.

## Build

```
go mod tidy
go build -o ipamthing .
```

## Run

```
./ipamthing -addr :8080 -db ipamthing.db
```

Open <http://localhost:8080>. First visit prompts to create an admin user.

### ICMP privileges

Unprivileged ICMP needs one of:

- `setcap cap_net_raw+ep ./ipamthing` (persistent)
- Adding the running user's GID to `net.ipv4.ping_group_range`:
  ```
  sudo sysctl -w net.ipv4.ping_group_range="0 2147483647"
  ```
- Running as root (not recommended)

Without ICMP, scanning falls back to ARP table reads + TCP connect probes on common ports. You'll see a notice on the subnet page if ICMP is disabled.

### systemd unit

```ini
[Unit]
Description=ipamthing
After=network-online.target

[Service]
ExecStart=/usr/local/bin/ipamthing -addr :8080 -db /var/lib/ipamthing/ipamthing.db
User=ipamthing
Group=ipamthing
AmbientCapabilities=CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_RAW
StateDirectory=ipamthing

[Install]
WantedBy=multi-user.target
```

## Flags

- `-addr` — HTTP listen address (default `:8080`)
- `-db` — SQLite database path (default `ipamthing.db`)
