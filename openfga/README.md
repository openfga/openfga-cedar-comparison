# OpenFGA Authorization Example

This is a complete example of using OpenFGA for authorization in a Go application, demonstrating document management permissions with relationship-based access control.

## What This Example Demonstrates

- **OpenFGA Authorization**: Using OpenFGA's relationship-based access control model
- **Entity Relationships**: Organizations, users, documents, folders with ownership and permissions  
- **Zanzibar-style Authorization**: Google Zanzibar-inspired fine-grained permissions
- **Go Integration**: Clean Go code structure using OpenFGA Go SDK

## Authorization Model

OpenFGA uses a relationship-based model where permissions are derived from relationships between entities:

### Entity Types
- **user**: Individual users in the system
- **organization**: Companies or groups  
- **folder**: Document containers with hierarchical permissions
- **document**: Files with inherited and explicit permissions

### Relations
- **member**: User membership in organizations
- **owner**: Full control over resources
- **editor**: Can modify resources  
- **viewer**: Can read resources
- **parent_folder**: Hierarchical relationship for inheritance

### Permission Rules
1. **Organization Access**: `organization.member` can view organization documents
2. **Ownership**: `resource.owner` has full permissions
3. **Explicit Permissions**: Direct `document.editor` or `document.viewer` relationships
4. **Folder Inheritance**: `folder.editor` can edit contained documents

## Quick Start

### Prerequisites
- Go 1.19+
- Docker and Docker Compose
- curl (for setup script)

### Setup

1. **Run the setup script (recommended):**
   ```bash
   ./setup.sh
   ```
   This automatically:
   - Starts OpenFGA and PostgreSQL
   - Creates store and authorization model
   - Writes test tuples
   - Builds the Go application

2. **Manual setup (alternative):**
   ```bash
   # Start services
   docker-compose up -d
   
   # Install dependencies
   go mod tidy
   
   # Build application
   go build -o openfga-check main.go
   
   # Set store ID (from setup script output)
   export OPENFGA_STORE_ID=<store-id>
   ```

### Usage

Test different authorization scenarios:

```bash
# Source environment variables
source .env

# Alice can view doc1 (she's the owner)
./openfga-check alice doc1
# ✅ ALLOWED: alice can view doc1

# Charlie can view doc2 (organization member + explicit viewer permission)
./openfga-check charlie doc2  
# ✅ ALLOWED: charlie can view doc2

# David cannot view doc1 (different organization, no permissions)
./openfga-check david doc1
# ❌ DENIED: david cannot view doc1

# Bob can view doc4 (explicit editor permission)
./openfga-check bob doc4
# ✅ ALLOWED: bob can view doc4
```

## Code Structure

- **`main.go`**: OpenFGA authorization checker
- **`document-management.fga`**: OpenFGA authorization model in DSL format  
- **`document-management-tuples.yaml`**: Relationship tuples (test data)
- **`document-management.fga.yaml`**: Test cases for the authorization model
- **`docker-compose.yml`**: OpenFGA + PostgreSQL setup
- **`setup.sh`**: Automated setup script

## Key Functions

### `checkAuthorization(client, userID, documentID)`
- Creates OpenFGA check request: `user:alice can_view document:doc1`
- Returns boolean decision from OpenFGA evaluation
- Leverages OpenFGA's relationship graph traversal

## Test Data

The example includes the same test data as the Cedar example for comparison:
- **Organizations**: org1 (Tech Corp), org2 (Marketing Inc)  
- **Users**: alice, bob, charlie (org1); david, eve (org2)
- **Documents**: doc1-doc4 with various ownership and permission structures
- **Relationships**: Organization membership, ownership, explicit permissions

## OpenFGA vs Cedar Comparison

| Aspect | OpenFGA | Cedar |
|--------|---------|-------|
| **Model** | Relationship-based (Zanzibar) | Policy-based (ABAC) |
| **Data Loading** | Relationships stored in OpenFGA | Entity attributes loaded at request time |
| **Evaluation** | Graph traversal | Policy evaluation engine |
| **Performance** | Optimized for relationship queries | Optimized for policy evaluation |
| **Flexibility** | Schema evolution through relationships | Rich policy language |

## OpenFGA Features Demonstrated

1. **Hierarchical Permissions**: Folder permissions inherit to documents
2. **Computed Relations**: `can_view` computed from `viewer` relation
3. **Union Relations**: Multiple paths to same permission
4. **Tuple-to-Userset**: Organization membership grants document access
5. **Relationship Traversal**: `parent_folder.editor` grants document edit rights

## Production Considerations

- **Relationship Indexing**: OpenFGA automatically indexes relationships
- **Caching**: Built-in caching for relationship lookups
- **Scalability**: Designed for high-throughput authorization
- **Consistency**: Strong consistency for relationship updates
- **Monitoring**: Built-in metrics and observability

## OpenFGA Playground

Access the web UI at http://localhost:3001 to:
- Visualize the authorization model
- Test relationship queries interactively  
- Debug authorization decisions
- Explore the relationship graph

## Cleanup

```bash
docker-compose down -v  # Removes containers and volumes
```

## Learning More

- [OpenFGA Documentation](https://openfga.dev/docs)
- [OpenFGA Go SDK](https://github.com/openfga/go-sdk)
- [Zanzibar Paper](https://research.google/pubs/pub48190/)
- [OpenFGA Playground](https://play.fga.dev/)
