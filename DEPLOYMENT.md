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
the published `ghcr.io/toxaris-nl/flashcards:latest` image, bind-mounts
`./config/flashcards` at `/etc/flashcards` and `./data/flashcards` at `/data`,
preserving JSON data across container recreation. Never remove or recreate
these host directories during an upgrade. The application uses
`/etc/flashcards/flashcards.toml` and may create or update it. Set
`ADMIN_PASSWORD_HASH` and `SESSION_SECRET` in the compose environment before
starting the service; both override the values saved in TOML. The container
uses Docker's default container user, so prepare both mounted directories as
usual for the server's other containers:

```sh
sudo mkdir -p config/flashcards data/flashcards
sudo chmod 750 config/flashcards data/flashcards
sudo chmod 640 config/flashcards/flashcards.toml 2>/dev/null || true
```

For a first install, run these commands before `./update.sh`; they also allow
the container to create the initial TOML file.

The compose file accepts `FLASHCARDS_CONFIG_DIR` and `FLASHCARDS_DATA_DIR` if
the stack uses fixed paths elsewhere; otherwise the defaults above are used.

The container listens on `:8080` and is mapped to host port `9990` by the
example.
Add its Caddy reverse-proxy route and any network configuration to the existing
operator-managed stack; this repository contains no Caddy configuration.

Run `./update.sh` on the server after a new image is available to pull and
recreate only the `flashcards` service. The script migrates the former
`./data` and `./flashcards.toml` locations when needed, and refuses to start
if an existing container has missing or empty storage. On a first install it
creates the host directories and allows the application to generate its default
configuration and data files.