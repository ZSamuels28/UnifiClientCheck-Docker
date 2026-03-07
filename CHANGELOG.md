# Changelog

All notable changes to this project will be documented in this file.

## [2.9.0] - 2026-03-06

### ⚡ Major: Refactored from PHP to Go

This is a complete rewrite of UniFiClientAlerts from PHP to Go. The application behavior is identical—all environment variables and features work the same way.

#### What's New
- **Performance**: Compiled Go binary is significantly faster and uses less memory
- **Deployment**: Single executable, no runtime dependencies (just Docker)
- **Code Quality**: Better organized with standard Go project structure (`/cmd`, `/internal`)
- **Reliability**: Go's built-in concurrency, error handling, and type safety

#### Breaking Changes
- **Docker Volume Path**: Changed from `/usr/src/myapp` → `/data`
  - Update your `docker-compose.yml` volume mount (see README migration guide)
  - All other environment variables remain unchanged

#### Technical Changes
- Reorganized to standard Go project layout:
  - `/cmd/unificlientalerts/` - Entry point
  - `/internal/config/` - Configuration loading
  - `/internal/database/` - SQLite operations
  - `/internal/notifier/` - Notification services
  - `/internal/unifi/` - UniFi API integration
- Upgraded Go version requirement from 1.22 to 1.24
- Upgraded Docker base from `golang:1.22` to `golang:1.24`
- GitHub Actions workflow now uses git tags for versioning (no more `version` file)
- Updated all action versions in CI/CD pipeline

#### Migration Notes
- **Docker users**: Just pull the latest image—no changes needed
- **Source builders**: Use `go build ./cmd/unificlientalerts` instead of `go build .`
- **Version tracking**: Releases are now tagged with git tags (e.g., `v2.9.0`) instead of a version file

#### Compatibility
- ✅ All notification services work as before (Telegram, Slack, Discord, MQTT, Webhook, etc.)
- ✅ All environment variables unchanged
- ✅ Database format unchanged (SQLite)
- ✅ Docker volume mounting unchanged

---

## [2.8] - 2026-03-05

### Features
- Original PHP implementation
- Full notification service support
- SQLite database for known MACs
- Docker deployment

[2.9.0]: https://github.com/ZSamuels28/UnifiClientCheck-Docker/releases/tag/v2.9.0
[2.8]: https://github.com/ZSamuels28/UnifiClientCheck-Docker/releases/tag/v2.8
