# Cedar Authorization Example

This is a complete example of using Amazon's Cedar policy engine for authorization in a Go application, demonstrating document management permissions with PostgreSQL data storage.

## What This Example Demonstrates

- **Cedar Policy Engine**: Using Cedar policies to define complex authorization rules
- **Entity Relationships**: Organizations, users, documents, folders with ownership and permissions
- **Database Integration**: Loading entity data from PostgreSQL to build Cedar entities
- **Go Integration**: Clean Go code structure with typed data and authorization functions

## Authorization Scenarios Covered

1. **Organization-based access**: Members can view documents in their organization
2. **Ownership-based access**: Document/folder owners have full access
3. **Permission-based access**: Explicit editor/viewer permissions on documents and folders
4. **Inheritance**: Folder permissions apply to contained documents

## Quick Start

### Prerequisites
- Go 1.19+
- Docker and Docker Compose (for PostgreSQL)

### Setup

1. **Start PostgreSQL with test data:**
   ```bash
   docker-compose up -d
   ```
   This automatically creates the database schema and loads test data.

2. **Install Go dependencies:**
   ```bash
   go mod init cedar-example
   go get github.com/cedar-policy/cedar-go
   go get github.com/lib/pq
   ```

3. **Build the application:**
   ```bash
   go build -o cedar-check main.go
   ```

### Usage

Test different authorization scenarios:

```bash
# Alice can view doc1 (she's the owner)
./cedar-check alice doc1
# ✅ ALLOWED: alice can view doc1

# Charlie can view doc2 (organization member + folder permission)
./cedar-check charlie doc2  
# ✅ ALLOWED: charlie can view doc2

# David cannot view doc1 (different organization, no permissions)
./cedar-check david doc1
# ❌ DENIED: david cannot view doc1

# Bob can view doc4 (explicit editor permission)
./cedar-check bob doc4
# ✅ ALLOWED: bob can view doc4
```

## Code Structure

- **`main.go`**: Complete Cedar authorization example
- **`policies.cedar`**: Cedar authorization policies
- **`schema.cedarschema`**: Cedar entity schema definition
- **`schema.sql`**: PostgreSQL database schema and test data
- **`docker-compose.yml`**: PostgreSQL setup

## Key Functions

### `queryEntityData(db, userID, documentID)`
- Executes optimized SQL query to load all entity relationship data
- Returns typed `EntityData` struct with user, document, folder, and permission information
- Uses CTEs to efficiently gather organization membership, document info, and permissions

### `checkAuthorization(policySet, data, userID, documentID)`
- Builds Cedar entities from the database data
- Creates Cedar authorization request
- Returns boolean decision from Cedar policy evaluation

## Test Data

The example includes realistic test data:
- **Organizations**: Tech Corp (org1), Marketing Inc (org2)
- **Users**: alice, bob, charlie (Tech Corp); david, eve (Marketing Inc)
- **Documents**: Various documents with different owners and permission structures
- **Permissions**: Mix of organization, ownership, and explicit permissions

## Database Schema

```sql
organizations -> users (via organization_members)
folders -> documents (via folder_id)
document_permissions, folder_permissions -> explicit user permissions
```

## Cedar Policies Explained

1. **Organization Access**: `principal.organization == resource.organization`
2. **Ownership**: `principal == resource.owner`
3. **Explicit Permissions**: `principal in resource.editors`
4. **Folder Inheritance**: `resource.parent_folder.viewers contains principal`

## Production Considerations

For production use, consider:
- Entity caching for better performance
- Policy hot-reloading
- Comprehensive error handling and logging
- Schema validation
- Performance monitoring
- Connection pooling for database access

## Cleanup

```bash
docker-compose down -v  # Removes containers and volumes
```

## Learning More

- [Cedar Documentation](https://docs.cedarpolicy.com/)
- [Cedar Go SDK](https://github.com/cedar-policy/cedar-go)
- [Cedar Policy Language Guide](https://docs.cedarpolicy.com/policies/syntax.html)

## What the SQL Query Loads

The policies require these data points for authorization decisions:

1. **Organization membership** (`organization_members` table)
2. **Document information** (ID, organization, folder, owner)
3. **Folder information** (ID, organization, owner) 
4. **Document permissions** (editors, viewers)
5. **Folder permissions** (editors, viewers - inherited by documents)

## The Comprehensive SQL Query

```sql
WITH user_org AS (
    SELECT organization_id as user_org_id
    FROM organization_members 
    WHERE user_id = $1 
    LIMIT 1
),
doc_info AS (
    SELECT d.id as doc_id, d.organization_id as doc_org_id, 
           d.folder_id, d.owner_id as doc_owner_id,
           f.organization_id as folder_org_id, f.owner_id as folder_owner_id
    FROM documents d
    LEFT JOIN folders f ON d.folder_id = f.id
    WHERE d.id = $2
),
doc_perms AS (
    SELECT dp.document_id, dp.user_id, dp.permission_type, 'document' as resource_type
    FROM document_permissions dp
    WHERE dp.document_id = $2
),
folder_perms AS (
    SELECT fp.folder_id as document_id, fp.user_id, fp.permission_type, 'folder' as resource_type
    FROM folder_permissions fp
    JOIN doc_info di ON fp.folder_id = di.folder_id
    WHERE di.folder_id IS NOT NULL
)
SELECT 
    uo.user_org_id,
    di.doc_id, di.doc_org_id, di.folder_id, di.doc_owner_id,
    di.folder_org_id, di.folder_owner_id,
    COALESCE(dp.user_id, '') as perm_user_id,
    COALESCE(dp.permission_type, '') as perm_type,
    COALESCE(dp.resource_type, '') as resource_type
FROM user_org uo
CROSS JOIN doc_info di
LEFT JOIN (
    SELECT * FROM doc_perms
    UNION ALL
    SELECT * FROM folder_perms
) dp ON true
```

## Cedar Policy Requirements

The Cedar policies need this data to evaluate:

- **Organization access**: `principal.organization == resource.organization`
- **Ownership**: `principal == resource.owner`  
- **Direct permissions**: `principal in resource.editors/viewers`
- **Folder inheritance**: `principal in resource.parent_folder.editors/viewers`

## Usage

```bash
go build -o cedar-check main.go

./cedar-check alice doc1    # ✅ Owner access
./cedar-check bob doc1      # ✅ Editor permission  
./cedar-check charlie doc2  # ✅ Folder viewer permission
./cedar-check david doc1    # ❌ Different organization
```

This version ensures that all Cedar policies can make accurate authorization decisions by having access to the complete entity relationship graph.
