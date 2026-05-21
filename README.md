# Plop

A personal file-and-text transfer tool. Send files or messages from a browser (or mobile PWA) to a paired Windows desktop client. Files are routed to local folders based on tags and a Windows toast notification fires on delivery.

## Components

| Component | Description |
|-----------|-------------|
| **Server** | Go HTTP server — handles auth, storage, and fan-out |
| **PWA** | Installable browser app for uploading files and text |
| **Desktop** | Windows tray app that receives and routes files |

## Running locally

```bash
# 1. Set up environment
cp .env.exmaple .env
# Add DATABASE_URL=postgres://user:pass@host:5432/plop_db
# Optionally: ALLOW_REGISTRATION=true (to create your account)

# 2. Start the server
go run ./cmd/server 
```

The server runs on port `8080` by default (`SERVER_PORT` to override). Migrations run automatically on startup — no separate DB setup step needed beyond creating the database and schema.

## Building the desktop client (Windows)

```bash
GOOS=windows GOARCH=amd64 go build -ldflags="-H windowsgui" -o plop.exe ./cmd/desktop
```

On first launch the setup wizard opens automatically. Enter your server URL and a pairing code generated from the web app's Settings page.

## Utilities

```bash
go run ./cmd/hashpw <password>  # bcrypt-hash a password for manual DB insertion
go run ./cmd/genicons           # regenerate PWA icons from assets/icons/plop1.png
```

## Deployment

Deployed via [LightHouse](https://github.com/LSariol/LightHouse) CI/CD. Requires:
- `DATABASE_URL` secret stored in [Cove](https://github.com/LSariol/Cove)
- `spark` external Docker network on the host
- Deployable code on `main`

