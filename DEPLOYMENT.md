# Deployment

The image is intended for GitHub Container Registry. This development machine
does not publish images; transfer the repository to a machine with GitHub
access and push to `main`.

Copy `flashcards.example.toml` to `flashcards.toml`. Set a long random
`admin.session_secret`. The application creates a usable development config if
the file is missing; replace the default credentials before production use. Do
not commit secrets. Environment variables
`ADMIN_USERNAME`, `ADMIN_PASSWORD_HASH`, and `SESSION_SECRET` override TOML.

Use `docker-compose.example.yml` as an external-stack service fragment. It uses
the published `ghcr.io/toxaris-nl/flashcards:latest` image, mounts
`./config/flashcards` as the read-only TOML file and `./data/flashcards` at
`/data`, preserving JSON data across container recreation. Set
`ADMIN_PASSWORD_HASH` and `SESSION_SECRET` in the compose environment before
starting the service; both override the values saved in TOML. The container
runs as UID 65532.

```sh
sudo chown 65532:65532 config/flashcards
sudo chmod 600 config/flashcards
```

The container listens on `:8080` and is mapped to host port `9990` by the
example.
Add its Caddy reverse-proxy route and any network configuration to the existing
operator-managed stack; this repository contains no Caddy configuration.

Run `./update.sh` on the server after a new image is available to pull and
recreate only the `flashcards` service.