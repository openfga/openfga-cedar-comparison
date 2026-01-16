# Cerbos Authorization Example

Implementation of the document management scenario using [Cerbos](https://cerbos.dev/) as the policy decision point (PDP). Policies live in YAML, the PDP runs as a container, and a small Go client calls the Cerbos HTTP API.

## What This Example Shows
- Derived roles to express ownership, explicit permissions, organization membership, and folder inheritance
- Resource policies for documents and folders matching the OpenFGA/Cedar examples using `derivedRoles` (not static `roles`)
- A tiny Go client that loads data from Postgres (same seed data as the Cedar example) and sends `CheckResources` requests to Cerbos
- A ready-to-run Docker Compose setup for the Cerbos PDP plus the seeded Postgres instance

## Quick Start
```bash
cd cerbos
./setup.sh            # Starts Cerbos PDP and builds the cerbos-check binary

./cerbos-check alice doc1   # ✅ Owner
./cerbos-check charlie doc2 # ✅ Document viewer
./cerbos-check david doc1   # ❌ Different org
./cerbos-check bob doc4     # ✅ Explicit editor
```

If you don't want to run `./setup.sh`, you can build the client directly with `go build -o cerbos-check main.go` (Postgres must already be up with the seeded data).

> Note: Cerbos requires `principal.roles` to be present. The client sets a default `["user"]` role, and the policies use `derivedRoles` to map attributes to semantic roles like `owner` and `org_member`.

## Running Policy Tests
Validate the policies with Cerbos' built-in test runner:
```bash
docker run --rm \
  -v "$PWD/policies:/policies:ro" \
  -v "$PWD/tests:/tests:ro" \
  ghcr.io/cerbos/cerbos:latest \
  compile /policies --tests /tests
```

## Files
- `docker-compose.yml`: Runs the Cerbos PDP with your local policies
- `config/conf.yaml`: Minimal Cerbos configuration using disk storage
- `policies/derived_roles.yaml`: Derived roles for ownership, permissions, org membership, and folder inheritance
- `policies/document.yaml`: Document authorization rules (view/edit/delete/share)
- `policies/folder.yaml`: Folder authorization rules (view/edit/delete/share)
- `tests/document_test.yaml`: Cerbos test suite covering the four documented scenarios (principals/resources fixtures)
- `main.go`: HTTP client that queries Postgres for the document/folder data, builds attributes, and calls the Cerbos PDP
- `setup.sh`: Helper to start Postgres (with seeded data), start the PDP, and build the client binary (`cerbos-check`)

## Data Model Used by the Client
The Go client uses the same sample data as the Cedar/OpenFGA examples:
- Organizations: `org1`, `org2`
- Users: `alice`, `bob`, `charlie` (org1) and `david`, `eve` (org2)
- Folders: `folder1` (org1, owner alice, viewer bob), `folder2` (org2, owner david, editor eve), `root_folder_org1` (org1, owner alice)
- Documents:
  - `doc1` (org1, owner alice, folder1)
  - `doc2` (org1, owner bob, folder1, viewer charlie)
  - `doc3` (org2, owner david, folder2)
  - `doc4` (org1, owner alice, root_folder_org1, editor bob, viewer charlie)

## How It Works
1. The derived roles map principal and resource attributes to semantic roles (`owner`, `editor`, `viewer`, `org_member`, `parent_*`).
2. Resource policies attach effects to actions using those derived roles.
3. The Go client gathers attributes for the requested document (including parent folder permissions), sends a `CheckResources` request to the PDP, and prints the result.
4. Cerbos evaluates the request against the YAML policies and returns `EFFECT_ALLOW` or `EFFECT_DENY`.
