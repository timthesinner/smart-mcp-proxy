# Source Code & Documentation References

This file lists the specific source code and documentation files referenced in the main architectural mindmap (`spec.md`).

## Section 1: Root
*   [`README.md`](./README.md)

## Section 2: Metadata & Core Info
*   [`docker/Dockerfile`](./docker/Dockerfile)
*   [`docker-compose.yml`](./docker-compose.yml)
*   [`docker-compose-dev.yml`](./docker-compose-dev.yml)
*   [`configs/example-config.json`](./configs/example-config.json)
*   [`internal/config/config.go`](./internal/config/config.go) (Loading Logic)
*   [`LICENSE`](./LICENSE)
*   [`README.md`](./README.md)
*   [`.roo/metadata`](./.roo/metadata)

## Section 3: Architecture Overview
*   [`cmd/proxy/proxy.go`](./cmd/proxy/proxy.go) (Core Component: ProxyServer)
*   [`cmd/proxy/http_mode.go`](./cmd/proxy/http_mode.go) (HTTP Mode)
*   [`cmd/proxy/command_mode.go`](./cmd/proxy/command_mode.go) (Command/STDIO Mode)
*   [`README.md`](./README.md)

## Section 4: Configuration (`internal/config`)
*   [`internal/config/config.go`](./internal/config/config.go)
*   [`internal/config/config_test.go`](./internal/config/config_test.go)
*   [`docs/configuration.md`](./docs/configuration.md)

## Section 5: Application Entrypoint (`cmd/proxy`)
*   [`cmd/proxy/main.go`](./cmd/proxy/main.go)

## Section 6: Proxy Logic (`cmd/proxy`)
*   [`cmd/proxy/proxy.go`](./cmd/proxy/proxy.go)

## Section 7: HTTP Mode Logic (`cmd/proxy`)
*   [`cmd/proxy/http_mode.go`](./cmd/proxy/http_mode.go)

## Section 8: Command/STDIO Mode Logic (`cmd/proxy`)
*   [`cmd/proxy/command_mode.go`](./cmd/proxy/command_mode.go)

## Section 9: Dockerization
*   [`docker/Dockerfile`](./docker/Dockerfile)
*   [`docker-compose.yml`](./docker-compose.yml)
*   [`docker-compose-dev.yml`](./docker-compose-dev.yml)
*   [`docs/docker-compose-dind-example.yml`](./docs/docker-compose-dind-example.yml)
*   [`README.md`](./README.md)
*   [`docs/usage.md`](./docs/usage.md)

## Section 10: Testing
*   [`internal/config/config_test.go`](./internal/config/config_test.go)
*   [`cmd/proxy/main_test.go`](./cmd/proxy/main_test.go)
*   [`cmd/proxy/proxy_test.go`](./cmd/proxy/proxy_test.go) (Note: File might not exist as listed)
*   [`cmd/proxy/http_mode_test.go`](./cmd/proxy/http_mode_test.go)
*   [`cmd/proxy/command_mode_test.go`](./cmd/proxy/command_mode_test.go)
*   [`README.md`](./README.md)

## Section 11: Documentation (`docs/`)
*   [`README.md`](./README.md)
*   [`docs/configuration.md`](./docs/configuration.md)
*   [`docs/usage.md`](./docs/usage.md)
*   [`docs/icon.png`](./docs/icon.png)
*   [`docs/docker-compose-dind-example.yml`](./docs/docker-compose-dind-example.yml)
*   [`README.md`](./README.md)

## Section 12: Dependencies
*   [`go.mod`](./go.mod)