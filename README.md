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

## CI / Releases

GitHub Actions (`.github/workflows/`) runs on every push to `main` and every
pull request:

- `ci.yml` — `make vet` (gofmt + go vet), build, tests, race tests; plus
  advisory `govulncheck` and Docker image build/smoke-test jobs.
- `sonarqube.yml` — [SonarQube Cloud](https://sonarcloud.io/project/overview?id=AlteredParadox_ipamthing)
  analysis with Go coverage. Needs the `SONAR_TOKEN` repo secret; rule
  exceptions live in `sonar-project.properties`, not the Cloud UI.
- `release.yml` — on a `vX.Y.Z` tag, builds static binaries for
  linux/amd64, linux/arm64 and windows/amd64 and publishes them with a
  `SHA256SUMS` file as a GitHub Release, and pushes the container image to
  `ghcr.io/alteredparadox/ipamthing` (linux/amd64 + linux/arm64). A manual
  run of the workflow is a dry run: it builds everything and uploads
  workflow artifacts without creating a Release or pushing an image.

Dependabot keeps Go modules, the pinned actions and the Docker base images
current (weekly, grouped).

To run the checks locally: `make vet license-check test`. To run a Sonar
analysis locally: `make sonar` (requires `sonar-scanner` on PATH and
`SONAR_TOKEN` exported).

## Flags

- `-addr` — HTTP listen address (default `:8080`)
- `-db` — SQLite database path (default `ipamthing.db`)
- `-setup-token` — one-time token required for first-run `/setup` (random
  and logged if empty and no users exist)
- `-trust-proxy` — honor `X-Forwarded-Proto` / `X-Forwarded-For` from a
  reverse proxy
- `-event-retention-days` — purge scan events older than N days (default
  `90`, `0` keeps forever)
- `-version` — print the version and exit

## Build

Requires Go 1.26+. `modernc.org/sqlite` is pure-Go (no cgo), so cross-compiles
cleanly to any target.

```
go mod tidy
make build        # stamps the version from `git describe`; `go build .` reports "dev"
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

### 1. Get a binary onto the server

Download the latest release binary for your architecture from
[GitHub Releases](https://github.com/AlteredParadox/ipamthing/releases)
(`ipamthing-linux-amd64` or `ipamthing-linux-arm64`; verify against
`SHA256SUMS`), or cross-compile on your dev box:

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

## Docker

Every release publishes a multi-arch image (linux/amd64, linux/arm64) to
GitHub Container Registry; `docker-compose.yml` uses it. The runtime image
is `alpine:3.24 + iproute2 + ipamthing` (~20 MB) running as the unprivileged
`ipamthing` user.

```
docker pull ghcr.io/alteredparadox/ipamthing:latest   # or :0.2.0 to pin
docker compose up -d
docker compose logs ipamthing        # read the first-run setup token
```

To build from source instead, swap `image:` for `build: .` in the compose
file (or `docker build -t ipamthing .`); the Dockerfile stamps the version
from the `VERSION` build arg.

The compose file uses **host networking** so the scanner sees the real LAN
and per-IP rate limiting sees real client addresses. If you'd rather run on
a bridge network behind a reverse proxy, remove `network_mode: host` and add
`-trust-proxy` to the `command:`.

Key compose bits:

- `cap_add: - NET_RAW` — raw ICMP sockets work without running as root.
- Persistent `ipamthing-data` volume mounted at `/var/lib/ipamthing`.
- Flags are passed as `command:` entries; uncomment `-setup-token=…`,
  `-trust-proxy`, or `-event-retention-days=…` as needed.

### Portainer + macvlan / ipvlan

If you run containers on a Portainer-managed macvlan (or ipvlan) network
so each container gets its own LAN IP, swap `network_mode: host` for an
explicit network reference. `-trust-proxy` is unnecessary — the container
sees real client IPs directly. No `ports:` mapping is needed; the
container is reachable at `<container-lan-ip>:8080`.

A ready-made variant lives at `docker-compose.portainer.yml` in the repo.
Point Portainer's "Compose path" at that file when creating the stack and
set these stack environment variables:

- `MACVLAN_NETWORK` — name of your pre-created macvlan network (default: `lan`).
- `IPAMTHING_IP` — optional static LAN IP for the container; leave unset
  to let the network's IPAM pool assign one.

For a Git-backed Portainer stack: Stacks → Add stack → Repository →
`https://github.com/AlteredParadox/ipamthing.git`, reference
`refs/heads/main`, compose path `docker-compose.portainer.yml`. Enable
GitOps updates (poll or webhook) for auto-redeploy on push.

Note the macvlan / ipvlan gotcha: the Docker host itself can't reach the
container's LAN IP (parent/child isolation). Browse from a different
machine on the LAN, or use Portainer's built-in container terminal / logs.

To bypass compose and run directly:

```
docker run -d --name ipamthing \
  --network host --cap-add NET_RAW \
  -v ipamthing-data:/var/lib/ipamthing \
  ghcr.io/alteredparadox/ipamthing:latest
```

### 4. Upgrading

```
# on dev box: download the new release binary, or rebuild
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o ipamthing-linux-amd64 .
scp -P <port> ipamthing-linux-amd64 <user>@<host>:/tmp/

# on server
sudo install -o root -g root -m 0755 /tmp/ipamthing-linux-amd64 /usr/local/bin/ipamthing
sudo systemctl restart ipamthing
```

The SQLite schema auto-migrates on startup (idempotent `ALTER TABLE` guards).

## License

ipamthing is free software under the **GNU Affero General Public License v3
or later** ([`LICENSE`](LICENSE)).

The AGPL's §13 obligation is to offer the Corresponding Source to everyone who
interacts with the program *over a network*, so these three routes are
deliberately served **without login** (and before first-run setup), since a
login wall would defeat the offer:

| Route | Serves |
| --- | --- |
| `/license` | the AGPL text embedded in this binary |
| `/third-party-licenses` | every linked dependency's licence and notices |
| `/source` | redirects to the Corresponding Source, pinned to the running version |

They are linked from the page footer next to the version. A stamped release
points `/source` at its own tag, so the source you are offered is the source
that is running; an unstamped build can only point at the repository.

`THIRD_PARTY_LICENSES.md` is generated from the modules **actually linked into
the binary** (read back off a probe build with `go version -m`):

```sh
make notices        # regenerate (also runs as part of `make build`)
make notices-check  # fail if the committed copy is stale
```

`notices-check` gates the release workflow rather than every PR, because
Dependabot bumps go.mod but cannot regenerate the file. Every first-party
source file carries the AGPL header; `make license-check` enforces that on
every PR and `make license-headers` stamps any new file.
