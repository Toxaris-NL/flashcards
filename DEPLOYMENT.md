# Deployment

The image is intended for GitHub Container Registry. This development machine
does not publish images; transfer the repository to a machine with GitHub
access and push to `main`.

Copy `flashcards.example.toml` to `flashcards.toml`. Set a long random
`admin.session_secret`. The application creates a usable development config if
the file is missing; replace the default credentials before production use. Do
not commit secrets. Environment variables
`ADMIN_USERNAME`, `ADMIN_PASSWORD_HASH`, and `SESSION_SECRET` override TOML.

Use `docker-compose.example.yml` as an external-stack service fragment. Set
`FLASHCARDS_IMAGE` to the published, valid lowercase image tag. It mounts the
TOML file and a host `data` directory at `/data`, preserving JSON data across
container recreation. The TOML mount must be writable because the admin dashboard
persists password changes to `admin.password_hash`. The container runs as UID
65532, so grant that account write access to the host config file, for example:

```sh
sudo chown 65532:65532 flashcards.toml
sudo chmod 600 flashcards.toml
```

When `ADMIN_PASSWORD_HASH` is set as an environment override, change that
environment value instead; it takes precedence over a password saved to TOML.

The container listens on the configured internal address, normally `:8080`.
Add its Caddy reverse-proxy route and any network configuration to the existing
operator-managed stack; this repository contains no Caddy configuration.

Run `./update.sh` on the server after a new image is available to pull and
recreate only the `flashcards` service.