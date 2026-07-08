# Commissary

A personal backend for storing and organizing files — documents, photos, videos, audio, etc.
Written in Go, structured with **Hexagonal architecture (Ports & Adapters)** and **DDD**.

## Core idea

Metadata and bytes are stored separately:

- **Metadata** (name, owner, MIME type, size, checksum, folder, tags) is modeled in the
  domain and persisted via an `AssetRepository`.
- **Bytes** (the "blob") stream through a `BlobStore` port so large files are never fully
  buffered in memory. Local filesystem now; S3/MinIO later — same interface.

## Architecture

Dependencies point **inward**. The domain knows nothing about the outside world; adapters
depend on the core, never the reverse.

```
                 inbound adapters                    outbound adapters
                 (driving)                           (driven)
   HTTP / gRPC  ─────────────►  application  ─ports─►  BlobStore  (localfs / s3)
   clients                      (use cases)            AssetRepository (postgres)
                                     │
                                     ▼
                                  domain
                              (pure business)
```

## Layout

```
cmd/commissary/            Composition root: wires adapters into services, serves.
internal/
  domain/asset/            Pure domain: Asset aggregate, value objects, errors. No I/O.
  application/
    asset/                 Use cases (Upload, Download, ...). The driving side.
    port/                  Outbound port interfaces (BlobStore, AssetRepository).
  adapter/
    inbound/http/          HTTP/JSON transport (streaming/multipart uploads).
    inbound/grpc/          gRPC transport (streaming RPCs).
    outbound/blob/localfs/ BlobStore on the local filesystem (implemented).
    outbound/blob/s3/      BlobStore on S3/MinIO (planned).
    outbound/persistence/postgres/  AssetRepository on PostgreSQL (planned).
  config/                  Runtime configuration.
api/openapi/               OpenAPI/Swagger spec for the HTTP API.
api/proto/                 Protobuf definitions for the gRPC API.
migrations/                SQL schema migrations.
configs/                   Example config files.
deployments/               Docker / compose / deployment manifests.
scripts/                   Dev and CI helper scripts.
test/                      Integration / end-to-end tests.
```

### Dependency rule
- `domain` imports nothing from other internal packages.
- `application` imports `domain` and `application/port` only.
- `adapter/*` imports `domain` + `application` (implements ports / calls use cases).
- `cmd/commissary` is the only package that imports concrete adapters.

## Getting started

```sh
go build ./...
go run ./cmd/commissary
```

Other languages may be added later for specific components (e.g. media processing); each
would sit behind a port so the Go core stays independent of them.
