#!/bin/sh
set -eu

# Keep storage outside the container. Migrate the paths used by older releases
# before Compose resolves the bind mounts, so an upgrade cannot start empty.
config_dir=${FLASHCARDS_CONFIG_DIR:-./config/flashcards}
data_dir=${FLASHCARDS_DATA_DIR:-./data/flashcards}
container_exists=0

if docker inspect flashcards >/dev/null 2>&1; then
	container_exists=1
fi

if [ ! -e "$config_dir/flashcards.toml" ] && [ -f ./flashcards.toml ]; then
	mkdir -p "$config_dir"
	cp -p ./flashcards.toml "$config_dir/flashcards.toml"
fi

if [ ! -d "$data_dir" ] || ! find "$data_dir" -mindepth 1 -print -quit | grep -q .; then
	if [ -f ./data/kids.json ] || [ -d ./data/content ] || [ -d ./data/progress ]; then
		mkdir -p "$data_dir"
		for item in ./data/kids.json ./data/content ./data/progress ./data/admin-settings.json; do
			if [ -e "$item" ]; then
				cp -a "$item" "$data_dir/"
			fi
		done
	elif [ "$container_exists" -eq 1 ]; then
		printf '%s\n' "Refusing to update: existing container has empty data storage: $data_dir" >&2
		exit 1
	else
		mkdir -p "$data_dir"
	fi
fi

if [ ! -f "$config_dir/flashcards.toml" ]; then
	if [ "$container_exists" -eq 1 ]; then
		printf '%s\n' "Refusing to update: existing container has missing config: $config_dir/flashcards.toml" >&2
		exit 1
	fi
	mkdir -p "$config_dir"
fi

docker compose pull flashcards
docker compose up -d flashcards