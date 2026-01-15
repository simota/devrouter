# DevRouter Configuration File Generation Prompt

Use the following prompt to generate DevRouter configuration for any repository.

---

## Prompt

```
You are an expert at generating DevRouter configuration files.

## What is DevRouter
DevRouter is a local development router that uses Traefik as a reverse proxy to make multiple services accessible via Host-based URLs.
- Example: `http://mystack-api.localtest.me`, `http://mystack-web.localtest.me`

## Task
Analyze the codebase in the specified directory and generate the following two files:

1. **devrouter.yaml** - DevRouter configuration file
2. **docker-compose.devrouter.yml** - Docker Compose file for Traefik integration

## Analysis Steps

### 1. Check Project Structure
- Monorepo or single app (pnpm-workspace.yaml, package.json workspaces, go.work)
- Existing docker-compose.yml
- Service/app directory structure (apps/, services/, packages/)

### 2. Detect Each Service
- Framework (Next.js, Vite, Hono, Express, Go, etc.)
- Port number (package.json scripts, config files, source code)
- Start command
- Health check endpoint (/health, /api/health, /)

### 3. Check Dependencies
- Database (PostgreSQL, MySQL, MongoDB)
- Cache (Redis)
- Message queue (Kafka, RabbitMQ)
- Other infrastructure services

### 4. Check Services with Multiple HTTP Ports
The following services expose multiple HTTP ports and require separate Traefik routers for each:
- MinIO: 9000 (S3 API) + 9001 (Console UI)
- RabbitMQ: 15672 (Management UI)
- Elasticsearch: 9200 (HTTP API)
- MailHog: 8025 (Web UI)

## Output Format

### devrouter.yaml
```yaml
stack: <project-name>
compose: docker-compose.devrouter.yml
domain: localtest.me

tls:
  enabled: true
  certFile: ~/.devrouter/certs/localtest.me.pem
  keyFile: ~/.devrouter/certs/localtest.me-key.pem

defaults:
  env:
    HOST: "0.0.0.0"

overrides:
  <service-name>:
    healthcheck:
      endpoint: <health-check-path>
      method: GET
      timeout: 5000
      status: [200]
```

### docker-compose.devrouter.yml
```yaml
services:
  <service-name>:
    build:
      context: .
      dockerfile: <dockerfile-path>
    # or
    image: <image-name>
    depends_on:
      <dependency-service>:
        condition: service_healthy
    environment:
      - <environment-variable>
    labels:
      - devrouter.enabled=true
      - traefik.enable=true
      - traefik.http.routers.<stack>-<service>.rule=Host(`<stack>-<service>.localtest.me`)
      - traefik.http.routers.<stack>-<service>.entrypoints=web
      - traefik.http.routers.<stack>-<service>-secure.rule=Host(`<stack>-<service>.localtest.me`)
      - traefik.http.routers.<stack>-<service>-secure.entrypoints=websecure
      - traefik.http.routers.<stack>-<service>-secure.tls=true
      - traefik.http.services.<stack>-<service>.loadbalancer.server.port=<port>
      - traefik.docker.network=devrouter_net
    networks:
      - devrouter_net
      - default

  # Example of service with multiple HTTP ports (MinIO)
  minio:
    image: minio/minio:latest
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    command: server /data --console-address ":9001"
    labels:
      - devrouter.enabled=true
      - traefik.enable=true
      # S3 API (port 9000)
      - traefik.http.routers.<stack>-minio.rule=Host(`<stack>-minio.localtest.me`)
      - traefik.http.routers.<stack>-minio.entrypoints=websecure
      - traefik.http.routers.<stack>-minio.tls=true
      - traefik.http.routers.<stack>-minio.service=<stack>-minio
      - traefik.http.services.<stack>-minio.loadbalancer.server.port=9000
      # Console UI (port 9001)
      - traefik.http.routers.<stack>-minio-console.rule=Host(`<stack>-minio-console.localtest.me`)
      - traefik.http.routers.<stack>-minio-console.entrypoints=websecure
      - traefik.http.routers.<stack>-minio-console.tls=true
      - traefik.http.routers.<stack>-minio-console.service=<stack>-minio-console
      - traefik.http.services.<stack>-minio-console.loadbalancer.server.port=9001
      - traefik.docker.network=devrouter_net
    networks:
      - devrouter_net

networks:
  devrouter_net:
    external: true
  default:
    driver: bridge

volumes:
  # Add as needed
```

## Important Rules

1. **Apply Traefik labels only to HTTP services** - Do not add `devrouter.enabled=true` to gRPC or TCP-only services
2. **Health checks** - Detect existing health endpoints and include them in the configuration
3. **Environment variables** - Update inter-service communication URLs to DevRouter URLs (e.g., `http://mystack-api.localtest.me`)
4. **Networks** - Connect all services to both `devrouter_net` and `default` networks
5. **Dependencies** - Set up `depends_on` with `condition: service_healthy` appropriately
6. **Multiple HTTP ports** - When a single service exposes multiple HTTP ports, configure separate Traefik routers for each port
   - Router names: `<stack>-<service>`, `<stack>-<service>-<suffix>` (e.g., `mystack-minio`, `mystack-minio-console`)
   - Hostnames: `<stack>-<service>.localtest.me`, `<stack>-<service>-<suffix>.localtest.me`
   - Host port mappings (`ports:`) are not needed when accessing through Traefik

## Target Directory
<specify the directory path here>

Analyze the directory above and generate devrouter.yaml and docker-compose.devrouter.yml.
```

---

## Usage Example

```
You are an expert at generating DevRouter configuration files.

[... full prompt above ...]

## Target Directory
/Users/username/repos/my-project

Analyze the directory above and generate devrouter.yaml and docker-compose.devrouter.yml.
```
