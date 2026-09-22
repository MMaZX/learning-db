# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# CGO_ENABLED=0: modernc.org/sqlite es Go puro, no necesita cgo/musl-gcc.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/db-intelligence-mcp ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/db-intelligence-mcp /app/db-intelligence-mcp
COPY knowledge ./knowledge

# La metadata (SQLite) debe persistir en un volumen externo, no en la imagen.
VOLUME ["/app/data"]

ENV MCP_KNOWLEDGE_PATH=/app/knowledge
ENV MCP_DATA_PATH=/app/data

# Sin credenciales embebidas: DB_* se inyectan en tiempo de ejecución.
ENTRYPOINT ["/app/db-intelligence-mcp"]
