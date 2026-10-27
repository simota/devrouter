# DevRouter

[![Go Version](https://img.shields.io/badge/Go-1.20+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A local development router that eliminates port conflicts without modifying your applications. Run multiple monorepo services (`web/admin/api`) simultaneously across multiple projects with stable, consistent URLs.

## Features

- **Zero App Changes** - Run services in parallel without modifying port configurations
- **Host-based URLs** - Access services via fixed URLs like `https://<stack>-web.localtest.me`
- **Automatic Routing** - Traefik-powered routing using Docker labels
- **HTTPS Support** - Secure local development with mkcert
- **Minimal Config** - Simple `devrouter.yaml` for overrides when needed
- **Web UI** - Real-time stack management, logs, and resource monitoring

## Quick Start

```bash
# Install (download binary from GitHub Releases or build from source)
# See Installation section below

# Start the global Traefik proxy
devrouter daemon up

# Start your monorepo stack
devrouter up /path/to/your/monorepo

# List running services
devrouter ls
# Example output: https://myrepo-web.localtest.me

# Stop the stack
devrouter down /path/to/your/monorepo
```

## Requirements

- macOS / Linux
- Docker (Docker Desktop or Colima)
- `docker` CLI with `docker compose`

### Build Requirements (for building from source)

- Go 1.20+
- Node.js 18+
- pnpm (`npm install -g pnpm`)

## Installation

### Download Binary (Recommended)

Download the latest binary from [GitHub Releases](https://github.com/simota/devrouter/releases):

```bash
# macOS (Apple Silicon)
curl -L https://github.com/simota/devrouter/releases/latest/download/devrouter_darwin_arm64.tar.gz | tar xz
sudo mv devrouter /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/simota/devrouter/releases/latest/download/devrouter_darwin_amd64.tar.gz | tar xz
sudo mv devrouter /usr/local/bin/

# Linux (amd64)
curl -L https://github.com/simota/devrouter/releases/latest/download/devrouter_linux_amd64.tar.gz | tar xz
sudo mv devrouter /usr/local/bin/
```

### Build from Source

> **Note**: `go install github.com/simota/devrouter/cmd/devrouter@latest` does not work because the Web UI frontend assets need to be built first and embedded into the binary.

```bash
git clone https://github.com/simota/devrouter.git
cd devrouter
make build
sudo mv devrouter /usr/local/bin/
```

Or install directly to `$GOPATH/bin`:

```bash
make install
```

Ensure `/usr/local/bin` or `$GOPATH/bin` is in your `PATH`.

## CLI Commands

| Command | Description |
|---------|-------------|
| `devrouter up [path]` | Start a stack from the specified path |
| `devrouter down [path\|stack]` | Stop a running stack |
| `devrouter ls` | List all running stacks and services |
| `devrouter logs <stack>[/<service>] [-f]` | View logs (with optional follow mode) |
| `devrouter open <stack>/<service>` | Open service URL in browser |
| `devrouter init [path]` | Generate `devrouter.yaml` from existing config |
| `devrouter exec <stack>/<service> [--] <cmd>` | Execute command in a service container |
| `devrouter doctor [path]` | Diagnose configuration issues |
| `devrouter daemon up` | Start the global Traefik proxy |
| `devrouter daemon down` | Stop the global Traefik proxy |
| `devrouter ui [--addr :19847]` | Launch the Web UI |

## Web UI

Launch with `devrouter ui` to access a browser-based management interface:

- **Stack Management** - View, restart, and stop running stacks
- **Real-time Logs** - WebSocket-powered log streaming with level filtering (ERROR/WARN/INFO/DEBUG)
- **Resource Monitoring** - CPU/memory usage with sparkline graphs
- **Dependency Graph** - SVG visualization of service dependencies
- **File Watcher** - Auto-restart on file changes with pattern matching
- **Service Templates** - Quick-add PostgreSQL, Redis, MongoDB, and more
- **Desktop Notifications** - Browser notifications for service status changes

Log streams close when the underlying Docker log command ends. Failures reading
the log output or running the command are reported in the log panel. Individual
log lines are limited to 1 MiB, including any Docker Compose prefix and excluding
LF/CRLF line endings. An oversized line reports an error and closes the stream
instead of silently stopping delivery. Selecting another stack closes the
previous log connection before opening the new one.

### Diagnostic snapshots

The Snapshot panel masks environment values by default. The same values can
also appear in the Compose file and dependency graph, so a masked export replaces
each nonempty environment value, the entire configuration content, and every
nonempty dependency URL/value with `******`. Dependency relationships, service
metadata, paths, health, resource statistics, and watch status remain available.
Empty or unavailable values stay empty; an unavailable environment response does
not disable masking of the other fields.

Select **Include raw env, config and dependency values** to include the original
API responses and configuration content. Values already redacted by the API
remain redacted in the environment list. The preview, Copy JSON, and Download
actions all use the selected export mode. Masked snapshots are not fully
anonymous: paths, service URLs, and other diagnostic metadata remain, so review
the export before sharing it.

## Configuration

Create `devrouter.yaml` in your repository root to customize behavior. Use `devrouter init` to auto-generate from existing `docker-compose.yaml` and `package.json`.

### Automatically discovered service names

Services discovered from workspaces use the workspace directory name, or an
`overrides.<workspace>.name` value, normalized to lowercase letters, digits, and
hyphens. The first occurrence of each normalized name keeps that name. Later
duplicates receive the first available numeric suffix starting at `-2`, skipping
all original names and names already assigned. Workspaces are processed in sorted
path order, so unchanged inputs produce the same names.

For example, `apps/web`, `packages/web`, and `services/web-2` become `web`,
`web-3`, and `web-2`. This keeps all three generated Compose definitions and their
default hostnames distinct. These names are also used when `devrouter init`
matches workspaces to Compose services. Services loaded directly from an external
Compose file keep the service keys in that file.

Projects affected by a suffix collision may see a corrected service name and
default URL on their next `devrouter up`. Check scripts and bookmarks referring
to those names. Existing explicit host overrides are preserved; this naming rule
does not validate custom hosts. Projects without such a collision keep their
existing names.

### Basic Configuration

```yaml
stack: myrepo
domain: localtest.me
```

### Port Overrides

```yaml
stack: myrepo
domain: localtest.me

defaults:
  env:
    HOST: "0.0.0.0"

overrides:
  apps/admin:
    port: 3001
  services/api:
    port: 4000
    env:
      PORT: "4000"
```

### External Compose File

Use your own `docker-compose.yaml` with DevRouter labels:

```yaml
stack: myrepo
compose: docker-compose.devrouter.yml
domain: localtest.me
```

Services must include DevRouter and Traefik labels:

```yaml
services:
  web:
    labels:
      - devrouter.enabled=true
      - traefik.enable=true
      - traefik.http.routers.myrepo-web.rule=Host(`myrepo-web.localtest.me`)
      - traefik.http.routers.myrepo-web.entrypoints=websecure
      - traefik.http.routers.myrepo-web.tls=true
      - traefik.http.services.myrepo-web.loadbalancer.server.port=3000
      - traefik.docker.network=devrouter_net
    networks:
      - devrouter_net
```

### File Watching

Auto-restart services on file changes:

```yaml
watch:
  enabled: true
  debounce: 500ms
  rules:
    - pattern: "*.ts"
      action: restart
    - pattern: "*.go"
      action: restart
```

The Web UI's `POST /api/stacks/{stackID}/watch/start` endpoint accepts an optional
JSON configuration with a 1 MiB request-body limit. Malformed JSON, invalid field
types, extra JSON values, and oversized bodies return HTTP 400 before changing
any watcher. An existing watcher keeps running if a request is rejected.

### AI-Assisted Configuration

Use an AI assistant (Claude, ChatGPT, etc.) to automatically generate configuration files for your project. See [`prompts/generate-config.md`](prompts/generate-config.md) for the full prompt.

Simply provide the prompt along with your project directory path, and the AI will analyze your codebase to generate:
- `devrouter.yaml` - DevRouter configuration
- `docker-compose.devrouter.yml` - Docker Compose with Traefik labels

## HTTPS Setup

Enable HTTPS for local development using [mkcert](https://github.com/FiloSottile/mkcert):

```bash
# Install mkcert and create local CA
mkcert -install

# Generate certificates
mkdir -p ~/.devrouter/certs
mkcert -cert-file ~/.devrouter/certs/localtest.me.pem \
       -key-file ~/.devrouter/certs/localtest.me-key.pem \
       "*.localtest.me"
```

Add TLS configuration to `devrouter.yaml`:

```yaml
stack: myrepo
domain: localtest.me
tls:
  enabled: true
  certFile: ~/.devrouter/certs/localtest.me.pem
  keyFile: ~/.devrouter/certs/localtest.me-key.pem
```

## How It Works

```
Browser
  |  Host: myrepo-web.localtest.me
  v
Host (80/443) --> Traefik (global) --> devrouter_net --> [myrepo-web:3000]
                                              |-------> [myrepo-admin:3001]
                                              |-------> [myrepo-api:4000]
```

1. **Global Traefik** listens on host ports 80/443
2. **Stack containers** join the shared `devrouter_net` network
3. **Host-based routing** directs requests to the correct `stack-service` container

### Registry consistency

The registry at `~/.devrouter/registry.json` is read as an independent snapshot
on every load. On macOS and Linux, registry changes from `up`, `down`, and the
UI's background sync use a short read-modify-write transaction protected by
`registry.json.lock`. Keep this lock file in place while DevRouter processes
are running. Updates use atomic replacement so readers see a complete file,
and concurrent operations preserve changes to unrelated stacks.

Each successful `up` records a new revision. A delayed `down` preserves a
replacement stack and reports a conflict. Background sync queries Docker
without the UI's polling cache and discards observations when the corresponding
registry record has changed. Docker commands run outside the registry lock;
operations on the same stack are not serialized end to end, and registry
conflict detection does not roll back Docker changes or guarantee that the
registry and Docker are always in agreement.

The optional `revision` field is compatible with existing version-1 registry
files; no migration is required. Restart all DevRouter CLI/UI processes on the
updated version to obtain these guarantees: older versions do not participate
in locking. Registry storage must support advisory file locks and atomic
rename. Atomic replacement does not provide a power-loss durability guarantee.

## Troubleshooting

| Issue | Solution |
|-------|----------|
| `docker info` fails | Start Docker/Colima, check docker context |
| Host not reachable | Ensure `HOST=0.0.0.0` in your app config |
| Wrong port detected | Explicitly set ports in `devrouter.yaml` |
| External compose not working | Add `devrouter.enabled=true`, Traefik labels, and `devrouter_net` |
| HMR not working | Check WebSocket proxy and host restrictions |

Use `devrouter doctor` to diagnose common configuration issues.

## Security Considerations

- **Network Exposure** - Setting `HOST=0.0.0.0` makes services accessible from your LAN
- **Docker Socket** - Mount Docker socket as read-only when possible

## Architecture

```
+----------------------------------------------------------+
|           devrouter CLI (cmd/devrouter)                  |
|  up | down | ls | logs | open | init | exec | doctor     |
+----+-----------------------------------------------------+
     |
     v (starts daemon if needed)
+------------------------+
|  Global Traefik        |  <-- Listens on 80/443
|  (daemon.go)           |  <-- Network: devrouter_net
+------------------------+
     ^
     |
+----+-------------------------------------------------+
|        Per-Stack Docker Compose (compose.go)         |
|  Services connected via devrouter_net with Traefik   |
|  labels for Host-based routing                       |
+------------------------------------------------------+

+------------------------------------------------------+
|  Web UI (server.go)                                  |
|  - Svelte frontend embedded via go:embed             |
|  - REST API (api_handlers.go)                        |
|  - WebSocket streams for logs/stats/watch            |
+------------------------------------------------------+
```

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

### Local validation

Build the frontend before running Go checks, because the Go binary embeds its
generated assets:

```bash
cd web
pnpm install --frozen-lockfile
pnpm run build
cd ..
make test
make lint
go test -race ./...
make build-go
```

Pull requests run frontend tests, Svelte/TypeScript checks, Go vet, Go tests with
the race detector, and builds on both Go 1.20 and the current stable Go release
on Linux, plus the stable Go release on macOS.
The log-stream regression tests use a temporary fake Docker executable and real
WebSocket connections, so they do not require a Docker daemon.

### Submitting changes

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

<details>
<summary>日本語 (Japanese)</summary>

## 概要

DevRouterは、アプリケーションを変更せずにポート衝突を解消するローカル開発用ルーターです。モノレポ内の複数HTTPサービス（web/admin/apiなど）を、複数プロジェクト同時に起動しても安定したURLでアクセスできます。

### 主な特長

- **アプリ無改造** - ポート設定を変更せずに並列起動
- **Hostベース固定URL** - `https://<stack>-web.localtest.me` 形式
- **Traefik自動ルーティング** - Dockerラベルで設定
- **HTTPS対応** - mkcertによるローカル証明書
- **Web UI** - リアルタイムのスタック管理・ログ・監視

詳細は上記の英語ドキュメントをご参照ください。

</details>
