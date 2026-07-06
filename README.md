# baru-reso-headless-controller

A self-hosted control plane for [baru-reso-headless-container](https://github.com/hantabaru1014/baru-reso-headless-container) that turns Resonite headless hosting into a multi-instance, multi-user service.

The official headless client is a console application: one terminal per instance, hand-edited config files, and access limited to whoever can reach the shell. This controller instead runs headless instances as Docker containers, so you and your team can spin up, operate, upgrade, and shut down any number of them from the browser — with group-based access control deciding who can manage what.

## Features

- **Host management**: start / stop / restart / delete headless containers, view their logs
  - Detects and pulls new container images automatically, and can upgrade running hosts on its own
- **Session management**: create / stop sessions, edit session parameters, invite / kick / ban users, change user roles
  - Schedule session operations with time- or condition-based triggers
  - Save worlds and download world binaries
- **Headless account management**: credentials, storage usage, friend requests and messaging
- **Access control**: group-based RBAC — see [docs/permissions.md](docs/permissions.md)
- **Log aggregation**: container logs are collected into PostgreSQL and browsable from the dashboard
- **Live updates**: server-side push keeps the dashboard in sync without reloading

## Requirements

- Docker with `network: host` available
- CPU arch: AMD64 or ARM64
- Access to a baru-reso-headless-container image registry

## Setup

The steps below bring up the controller, PostgreSQL, and friends with docker compose.
If you want to run on k8s or connect to an existing PostgreSQL, run setup.sh first and then adjust the compose files and `.env`.

1. Create an empty directory and cd into it
2. Run the setup script
   ```sh
   sh <(curl -s https://raw.githubusercontent.com/hantabaru1014/baru-reso-headless-controller/refs/heads/main/scripts/setup.sh)
   ```
   The script takes care of:
   - Downloading the required files (docker-compose.yml, brhcli, etc.)
   - Generating the `.env` file
   - Starting PostgreSQL / fluent-bit / RustFS
   - Running database migrations
3. Create an admin user
   ```sh
   ./brhcli user create <email> <password> <Resonite UserID>
   ```
4. Start the controller
   ```sh
   docker compose up -d
   ```
5. Done. The dashboard is available at http://localhost:8014/
   - The port can be changed via the `HOST` variable in `.env`
   - If you expose it to the internet, protect the endpoint with a reverse proxy, Cloudflare Zero Trust, or similar

## Upgrading

To upgrade a running deployment to the latest version, run the following in the directory you used for setup:

```sh
sh <(curl -s https://raw.githubusercontent.com/hantabaru1014/baru-reso-headless-controller/refs/heads/main/scripts/auto-upgrade.sh)
```

It downloads the latest compose files, images, and brhcli, runs database migrations, appends any newly required settings to `.env`, and recreates the containers.
Backing up important data before upgrading is recommended.

## Documentation

- [Development guide](docs/development.md) - architecture, dev environment setup, testing
- [Permission system](docs/permissions.md) - RBAC concepts and configuration
