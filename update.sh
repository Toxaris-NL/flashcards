#!/bin/sh
set -eu

docker compose pull flashcards
docker compose up -d flashcards