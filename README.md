
# Authorization Frameworks Comparison: Cedar vs OpenFGA vs Cerbos

> A comprehensive comparison of two modern authorization frameworks through a practical document management system example.

## Table of Contents
- [Overview](#overview)
- [The Problem](#the-problem)
- [Quick Start](#quick-start)
- [Architecture Comparison](#architecture-comparison)
- [Implementation Details](#implementation-details)
- [Trade-offs Analysis](#trade-offs-analysis)
- [When to Choose Each](#when-to-choose-each)

## Overview

This repository demonstrates three different approaches for implementing modern application authorization by comparing [OpenFGA](https://openfga.dev/), [Cedar](https://www.cedarpolicy.com), and [Cerbos](https://cerbos.dev/).

We implement authorization for a **multi-tenant document management system** with organizations that own folders containing documents - a common real-world scenario that showcases the strengths and trade-offs of each approach.

## The Problem

We need authorization for a document management system with these requirements:

### Business Rules
1. **Organization-based access**: Users can view documents in their organization
2. **Ownership**: Document/folder owners have full access (view, edit, delete, share)
3. **Explicit permissions**: Grant editor/viewer permissions on documents and folders
4. **Inheritance**: Folder permissions apply to contained documents

### Test Scenarios
- ✅ **alice can view doc1**: She's the owner
- ✅ **charlie can view doc2**: Organization member + folder viewer permission
- ❌ **david cannot view doc1**: Different organization, no permissions
- ✅ **bob can view doc4**: Explicit editor permission

## Quick Start

### Prerequisites
- Go 1.19+
- Docker and Docker Compose
- curl (for setup scripts)

### Try OpenFGA Example
```bash
cd openfga/
./setup.sh
./openfga-check alice doc1    # ✅ Owner access
./openfga-check david doc1    # ❌ Cross-organization denied
```

### Try Cedar Example  
```bash
cd cedar/
./setup.sh
./cedar-check alice doc1     # ✅ Owner access
./cedar-check david doc1     # ❌ Cross-organization denied
```

### Try Cerbos Example  
```bash
cd cerbos/
./setup.sh
./cerbos-check alice doc1    # ✅ Owner access
./cerbos-check david doc1    # ❌ Cross-organization denied
```

All three examples provide identical authorization decisions using different approaches.

## Architecture Comparison

### OpenFGA: Relationship-Based Authorization

```dsl.openfga
model
  schema 1.1

type user

type organization
  relations
    define member: [user]

type folder
  relations
    # define parent: [folder] -> Not added because Cedar does not support recursion
    define organization: [organization]
    define owner: [user]
    define editor: [user] or owner # or editor from parent 
    define viewer: [user] or editor or member from organization # or viewer from parent 

    define can_view: viewer
    define can_edit: editor
    define can_delete: owner
    define can_share: owner or editor

type document
  relations
    define organization: [organization]
    define parent_folder: [folder]
    define owner: [user]
    define editor: [user] or owner or editor from parent_folder
    define viewer: [user] or editor or viewer from parent_folder or member from organization

    define can_view: viewer
    define can_edit: editor
    define can_delete: owner
    define can_share: owner or editor
```

> OpenFGA supports recursion (e.g., inheriting folder permissions). Cedar does not, and while Cerbos can express recursive derived roles, we avoid recursion across all three examples for parity.

In Cedar you can define an entity schema that you can use to validate policies, but is not a requirement. You can find the schema we use for this example [here](cedar/schema.cedarschema). You define the authorization policies in the [Cedar language](https://docs.cedarpolicy.com/policies/syntax-policy.html):

```
// Document Management Authorization Policies

// Organization member can view organization documents
permit(
    principal,
    action == DocumentManagement::Action::"ViewDocument",
    resource
) when {
    principal.organization == resource.organization
};

// Organization member can view organization folders
permit(
    principal,
    action == DocumentManagement::Action::"ViewFolder",
    resource
) when {
    principal.organization == resource.organization
};

// Document owner can perform all actions on their documents
permit(
    principal,
    action in [
        DocumentManagement::Action::"ViewDocument",
        DocumentManagement::Action::"EditDocument",
        DocumentManagement::Action::"DeleteDocument",
        DocumentManagement::Action::"ShareDocument"
    ],
    resource
) when {
    principal == resource.owner
};

// Document editor can edit and share documents they edit
permit(
    principal,
    action in [
        DocumentManagement::Action::"EditDocument",
        DocumentManagement::Action::"ShareDocument"
    ],
    resource
) when {
    principal in resource.editors
};

// Document viewer can view documents
permit(
    principal,
    action == DocumentManagement::Action::"ViewDocument",
    resource
) when {
    principal in resource.viewers
};

// Folder owner can perform all actions on their folders
permit(
    principal,
    action in [
        DocumentManagement::Action::"ViewFolder",
        DocumentManagement::Action::"EditFolder",
        DocumentManagement::Action::"DeleteFolder",
        DocumentManagement::Action::"ShareFolder"
    ],
    resource
) when {
    principal == resource.owner
};

// Folder editor can view, edit, and share folders (but not delete)
permit(
    principal,
    action in [
        DocumentManagement::Action::"ViewFolder",
        DocumentManagement::Action::"EditFolder",
        DocumentManagement::Action::"ShareFolder"
    ],
    resource
) when {
    principal in resource.editors
};

// Folder editor can view, edit, and share documents in their folders
permit(
    principal,
    action in [
        DocumentManagement::Action::"ViewDocument",
        DocumentManagement::Action::"EditDocument",
        DocumentManagement::Action::"ShareDocument"
    ],
    resource
) when {
    principal in resource.parent_folder.editors
};

// Folder owner can view, edit, and share documents in their folders
permit(
    principal,
    action in [
        DocumentManagement::Action::"ViewDocument",
        DocumentManagement::Action::"EditDocument",
        DocumentManagement::Action::"ShareDocument"
    ],
    resource
) when {
    principal == resource.parent_folder.owner
};

// Folder viewers can view folders
permit(
    principal,
    action == DocumentManagement::Action::"ViewFolder",
    resource
) when {
    principal in resource.viewers
};

// Folder viewers can view documents in folders
permit(
    principal,
    action == DocumentManagement::Action::"ViewDocument",
    resource
) when {
    principal in resource.parent_folder.viewers
};
```

Both policies are equivalent and hopefully self-explanatory. The approaches are very different though. In OpenFGA permissions are defined in terms of relations, which lets you define all the different ways a user can get a permission in a single line (e.g. ` define viewer: [user] or editor or viewer from parent_folder or member from organization`) while navigating resources hierarchies, and in Cedar you need to define define multiple `permit` clauses.

### Cerbos: PDP with Derived Roles and YAML Policies

Cerbos policies live in YAML and are evaluated by a PDP that can be deployed as a sidecar or shared service. The same document/folder rules are expressed via derived roles and resource policies (note the use of `derivedRoles`, not static `roles`). Example (simplified):

```yaml
# policies/derived_roles.yaml
derivedRoles:
  name: document_management_roles
  definitions:
    - name: org_member
      condition: { match: { expr: request.principal.attr.organization == request.resource.attr.organization } }
    - name: owner
      condition: { match: { expr: request.resource.attr.owner == request.principal.id } }
    - name: parent_editor
      condition: { match: { expr: request.resource.attr.parent_folder != null && request.resource.attr.parent_folder.editors.contains(request.principal.id) } }

# policies/document.yaml
resourcePolicy:
  resource: document
  importDerivedRoles: [document_management_roles]
  rules:
    - actions: ["ViewDocument"]
      effect: EFFECT_ALLOW
      roles: [owner, editor, viewer, org_member, parent_owner, parent_editor, parent_viewer]
```

Clients send HTTP/gRPC `CheckResources` requests with principal/resource attributes. Our Cerbos sample loads the same SQL data as the Cedar example, builds the attribute map (including parent folder permissions), and asks the PDP for a decision.

## Key Architectural Differences

**OpenFGA: Service-Based Authorization**
- Runs as a separate service with its own database
- All authorization data stored in OpenFGA
- Single API call for authorization decisions
- Requires network roundtrip for each check

**Cedar: Library-Based Authorization** 
- Runs as an embedded library in your application
- Data retrieved from your existing databases
- No network calls, but requires you to load the data
- Authorization logic coupled with data access

**Cerbos: Service-Based PDP with YAML Policies**
- Runs as a PDP sidecar/shared service (no database when using disk policies)
- Policies stored as YAML (derived roles + resource rules)
- Caller loads resource/principal data and sends attributes over HTTP/gRPC
- Network roundtrip per decision; no policy coupling to application code
## Implementation Examples

### OpenFGA Authorization Check

```go
func checkAuthorization(fgaClient *client.OpenFgaClient, userID, documentID string) (bool, error) {
    body := client.ClientCheckRequest{
        User:     fmt.Sprintf("user:%s", userID),
        Relation: "can_view", 
        Object:   fmt.Sprintf("document:%s", documentID),
    }

    data, err := fgaClient.Check(context.Background()).Body(body).Execute()
    if err != nil {
        return false, fmt.Errorf("check request failed: %w", err)
    }

    return *data.Allowed, nil
}
```

### Cedar Authorization Check

```go
// 1. Load data from your database
data, err := queryEntityData(db, userID, documentID)
if err != nil {
    return false, err
}

// 2. Build Cedar entities from the data
entities := buildCedarEntities(data)

// 3. Create authorization request
request := cedar.Request{
    Principal: cedar.NewEntityUID("User", userID),
    Action:    cedar.NewEntityUID("Action", "ViewDocument"), 
    Resource:  cedar.NewEntityUID("Document", documentID),
}

// 4. Authorize with Cedar
decision, _ := cedar.Authorize(policySet, entities, request)
return decision == cedar.Allow, nil
```
In the Cedar example, we are using this SQL query to retrieve the data required to know if a user can view a document:

```sql
WITH user_org AS (
		SELECT organization_id as user_org_id
		FROM organization_members 
		WHERE user_id = $1 
		LIMIT 1
	),
	doc_info AS (
		SELECT d.id as doc_id, d.organization_id as doc_org_id, d.folder_id, d.owner_id as doc_owner_id,
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
		uo.user_org_id, di.doc_id, di.doc_org_id, di.folder_id, di.doc_owner_id, di.folder_org_id, di.folder_owner_id,
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

There are other ways to write a single or multiple queries and get a similar results. After you retrieve the data, you need to convert it to an instance of a Cedar Entity. The [cedar/main.go](cedar/main.go) program has the full example. The Cerbos example reuses the same query to build resource/principal attributes before calling the PDP (see [cerbos/main.go](cerbos/main.go)).

### Cerbos Authorization Check

```go
// 1. Load data from your database (same query as Cedar)
data, err := queryEntityData(db, userID, documentID)

// 2. Build Cerbos attributes
attrs := buildResourceAttributes(data)

// 3. Call Cerbos PDP over HTTP
req := checkRequest{ /* principal with organization attr, resource attrs, action */ }
resp, _ := client.Post(cerbosCheckEndpoint, "application/json", body)
decision := resp.Results[0].Actions["ViewDocument"].Effect == "EFFECT_ALLOW"
```

Cerbos keeps policy evaluation in a service, but—like Cedar—you still load business data from your database to populate request attributes.

## OpenFGA's Contextual Tuples

In general, when using OpenFGA, you will store all the data required to make authorization decisions in OpenFGA. When using Cedar or Cerbos, you'll store it in your application.

However, OpenFGA allows a hybrid model, where you can actually specify the data required to make the decision in [Contextual Tuples](https://openfga.dev/docs/interacting/contextual-tuples). Conceptually, you can do something equivalent to what the Cedar example shows, get all the data from a SQL database, and send it as part of the authorization request. 

It would not make sense to use OpenFGA that way, though. If in all scenarios you are going to first retrieve the data from your database, Cedar or Cerbos is a better option.

On the other hand, combining having data in OpenFGA AND sending contextual data gives you a lot of flexibility. If you can easily synchronize data to OpenFGA, you'd do that. When you can't, because data is not stored in a database (e.g. the content of an access token), or because synchronizing it is hard, you can send it as part of the request.

## Trade-offs Analysis

| Aspect | OpenFGA | Cedar | Cerbos |
|--------|---------|-------|--------|
| **Latency** | Network call required, optimized for relationship queries | No network call, but requires data loading | Network call to PDP; decision is fast, data loading on caller |
| **Complexity** | Simple API calls, easy integration | Complex data loading and entity building | YAML policies + derived roles; client must assemble attributes |
| **Maintainability** | Policy changes don't affect app code | Policy changes may require SQL changes | Policy-only changes rarely touch code; data loading remains in app |
| **Operations** | Requires running separate service + database | No additional infrastructure | Requires running PDP service (no DB when using disk storage) |
| **List Operations** | Native "list all documents user can view" | Requires custom SQL or partial evaluation | Requires custom SQL or indexing in your app |
| **Data Consistency** | Dual-write problem for data sync | Uses existing transactional data | Uses existing transactional data; no policy data store |
| **Recursion** | Supports modeling recursive permissions | Does not support recursive permissions | Supports recursive derived roles with care |

### Detailed Trade-offs

#### **Latency**
- **OpenFGA**: Network roundtrip required, but queries are optimized and cacheable
- **Cedar**: No network call, but data loading latency depends on query complexity
- **Cerbos**: Network roundtrip to PDP; decision is fast, and data loading latency lives in the caller

#### **Access Control Complexity**
- **OpenFGA**: Simple API calls - easily integrated into API gateways
- **Cedar**: Requires data retrieval and transformation - more complex integration
- **Cerbos**: Simple API calls with YAML policies; attribute assembly lives in the app

#### **Maintainability**  
- **OpenFGA**: Policy changes isolated from application code
- **Cedar**: Authorization logic coupled with database queries
- **Cerbos**: Policy changes rarely touch code; only attribute loading changes when data needs expand

#### **Operations**
- **OpenFGA**: Additional service to operate, but dedicated authorization infrastructure
- **Cedar**: No extra infrastructure, but higher database load
- **Cerbos**: PDP service to operate; disk policy store avoids extra data infra

#### **Reverse Queries**
- **OpenFGA**: Built-in [ListObjects](https://openfga.dev/docs/getting-started/perform-list-objects) and [ListUsers](https://openfga.dev/docs/getting-started/perform-list-users) APIs. The latency for those calls will heavily depend on the authorization model.
- **Cedar**: Requires encoding authorization logic in SQL, post-filtering results, or use the experimental [partial evaluation implementation](https://www.cedarpolicy.com/blog/partial-evaluation) to generate a filter for your local database.
- **Cerbos**: Requires custom SQL or indexing in your app; Cerbos focuses on decisioning, not data retrieval

## Performance Considerations

Given the differences in architecture, a performance comparison between the engines does not make sense:

- The raw Authorize call from Cedar will always be much faster than the equivalent OpenFGA or Cerbos operation, as it does not require a network call.
- Cerbos decisions are fast but do incur a network hop; there is no policy database when using disk storage.
- The overall performance will depend on how each system retrieves the data required to make the decision. OpenFGA is designed to optimize relationship traversal, while Cedar and Cerbos rely on your application to load the right attributes.

## When to Choose Each

### Choose OpenFGA When:
- You need **fine-grained permissions** with complex inheritance
- **List operations** are important ("show all documents user can view")
- You want **authorization data logic separate** from business logic 
- You require additional data when **additional data** when making authorization decisions
- Your authorization requirements are **relationship based** rather than attribute-based

### Choose Cedar When:
- You have **rich entity attributes** that drive decisions
- You want to **minimize infrastructure** complexity
- Your authorization is **primarily attribute-based** rather than relationship-based
- The application already has **all the data required to make authorization decisions**

### Choose Cerbos When:
- You want **policy-as-code** with a service-based PDP (sidecar/shared) rather than an embedded library
- Teams prefer **YAML policies with derived roles** and a simple check API
- You already load **business data from your DB** and need a centralized decision point
- You want **hot-reloadable policies** without redeploying application code

## Learning Resources

### Cedar
- [Cedar Documentation](https://docs.cedarpolicy.com/)
- [Cedar Go SDK](https://github.com/cedar-policy/cedar-go)
- [Cedar Policy Language Guide](https://docs.cedarpolicy.com/policies/syntax.html)
- [Cedarland Blog](https://cedarland.blog/)

### OpenFGA
- [OpenFGA Documentation](https://openfga.dev/)
- [OpenFGA Go SDK](https://github.com/openfga/go-sdk)  
- [Zanzibar Paper](https://research.google/pubs/pub48190/) - The original Google paper
- [OpenFGA Playground](https://play.fga.dev/) - Interactive modeling tool

### Cerbos
- [Cerbos Documentation](https://docs.cerbos.dev)
- [Cerbos Policies](https://docs.cerbos.dev/cerbos/latest/policies/overview.html)
- [Check Resources API](https://docs.cerbos.dev/cerbos/latest/api/cerboslsp.v1.html#checkresourcesrequest)
- [Cerbos GitHub](https://github.com/cerbos/cerbos)
