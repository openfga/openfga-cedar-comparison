#!/usr/bin/env bash
set -euo pipefail

echo "Starting Postgres with sample data..."
docker-compose -f ../cedar/docker-compose.yml up -d postgres

echo "Starting Cerbos PDP..."
docker-compose up -d

echo "Waiting for PostgreSQL to be healthy..."
until nc -z 127.0.0.1 5432; do
  echo "   Still waiting for PostgreSQL..."
  sleep 2
done

echo "Waiting for Cerbos PDP to be healthy..."
until curl -sf http://localhost:3592/_cerbos/health > /dev/null; do
  echo "   Still waiting for Cerbos..."
  sleep 2
done

echo "Building Go client..."
go build -o cerbos-check main.go

echo "Ready. PDP is on http://localhost:3592 and gRPC on :3593"
