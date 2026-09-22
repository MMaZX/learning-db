#!/usr/bin/env bash
set -e
cd /home/fulanito/development/mcp-bd
exec /home/fulanito/development/mcp-bd/dist/db-intelligence-mcp-linux-amd64 "$@"
