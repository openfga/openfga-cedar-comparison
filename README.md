
This repository demostrates two different approach for implementing modern application authorization comparing two open source tools: OpenFGA and Cedar.

We'll do it by implementing authorization for a multi-tenant document management system that defines 'organizations' that can own 'folders' with 'documents'.

Both Cedar and OpenFGA have their own domain specific languages to define authorization policies and schema.  

In OpenFGA model you define the schema and policies in a single model:

```dsl.openfga
model
  schema 1.1

type user

type organization
  relations
    define member: [user]

type folder
  relations
    define organization: [organization]
    define owner: [user]
    define editor: [user] or owner
    define viewer: [user] or editor or member from organization

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

In Cedar you can define an entity schema that you can use to validate policies, but is not a requirement. You can find the schema we use for this example [here](cedar/schema.cedarschema). You define the authorization policies in the Cedar language:

```
// Document Management Authorization Policies

// Organization member can view organization documents
permit(
    principal,
    action == DocumentManagement::Action::"ViewDocument",
    resource
) when {
    principal has organization &&
    resource has organization &&
    principal.organization == resource.organization
};

// Organization member can view organization folders
permit(
    principal,
    action == DocumentManagement::Action::"ViewFolder",
    resource
) when {
    principal has organization &&
    resource has organization &&
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
    resource has owner && principal == resource.owner
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
    resource has editors && principal in resource.editors
};

// Document viewer can view documents
permit(
    principal,
    action == DocumentManagement::Action::"ViewDocument",
    resource
) when {
    resource has viewers && principal in resource.viewers
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
    resource has owner && principal == resource.owner
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
    resource has editors && principal in resource.editors
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
    resource has parent_folder &&
    resource.parent_folder has editors &&
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
    resource has parent_folder &&
    resource.parent_folder has owner &&
    principal == resource.parent_folder.owner
};

// Folder viewers can view folders
permit(
    principal,
    action == DocumentManagement::Action::"ViewFolder",
    resource
) when {
    resource has viewers && principal in resource.viewers
};

// Folder viewers can view documents in folders
permit(
    principal,
    action == DocumentManagement::Action::"ViewDocument",
    resource
) when {
    resource has parent_folder &&
    resource.parent_folder has viewers &&
    principal in resource.parent_folder.viewers
};
```

Both policies are equivalent and hopefully self-explanatory. The approaches are very different though. In OpenFGA you can define all the different ways a user can get a permission in a single line (e.g. ` define viewer: [user] or editor or viewer from parent_folder or member from organization`), and in Cedar you could one define multiple `permit` clauses.

However, the main difference in both approaches is the application architecture:

- OpenFGA runs as a service, and it's a permission database. All the data required to make authorization decisions should be stored in OpenFGA. Making an access control check requires a network roundtrip.

- Cedar runs as a library, and you need to retrieve the data you need to make authorization decisions first, and then call Cedar to evaluate the policy.

We'll explore how to implement both to illustrate the trade offs of each approach. 

## Implementing the OpenFGA solution

- Deploy OpenFGA in a cluster, connected to a MySQL or Postgres database.
- Configure OpenFGA to use the Authorization model you want.
- Write authorization data to the OpenFGA database. This requires you to implement a data synchornization mechanism. A few options are described [here](https://auth0.com/blog/handling-the-dual-write-problem-in-distributed-systems/). 
- Call the OpenFGA Check API to know if a user can perform an action on a resource.

The `openfga/setup.sh` script runs OpenFGA + Postgres in docker, uploads the model and data to a database. The `openfga/main.go` program shows how to perform an authorization check, which is basically like:

```go
func checkAuthorization(fgaClient *client.OpenFgaClient, userID, documentID string) (bool, error) {
	// Create check request
	body := client.ClientCheckRequest{
		User:     fmt.Sprintf("user:%s", userID),
		Relation: "can_view",
		Object:   fmt.Sprintf("document:%s", documentID),
	}

	// Execute check
	data, err := fgaClient.Check(context.Background()).Body(body).Execute()
	if err != nil {
		return false, fmt.Errorf("check request failed: %w", err)
	}

	return *data.Allowed, nil
}
```
## Implementing the Cedar solution

- Cedar does not require any specific infrastructure. There are implementations for Go and Rust, and bindings to other languages like Java (.. what else? links)

- The call to the Cedar engine is pretty simple too:

```go
	decision, diagnostic := cedar.Authorize(policySet, entities, request)
```

- In the line above, the 'policySet' is the policy, the entities is the data required to make the decision, and the request includes the user/action/resource you want to authorize.

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

There are other ways to write a single query with the same results, or you can execute multiple queries. After you retrieve the data, you need to convert it to an instance of a Cedar Entity.

The [cedar/main.go](cedar/main.go) program has the full example.

## Trade Offs

### Latency

OpenFGA requires a network call, but the database queries are optimized for the specific query patterns, and OpenFGA can cache parts of the graph to resolve queries faster.

Cedar does not require a network call to evaluate a policy, but it requires loading the data. Total latency will depend on how expensive is to retrieve it. If it's a simple database call, it will be super fast. If it's a complex SQL query or if it requires data from multiple services, it will be slower.

### Access Control Checks Complexity

Performing Access Control checks in OpenFGA is very easy. You only call the `check` API, and OpenFGA has all the data required to answer the query. They can be easily integrated into an API Gateway, as there's no additional data required.

In Cedar, you first need to retrieve the data, and transform it to Cedar Entity object instances. This adds complexity when making authorizatoin checks, and makes it more difficult to integrate in API gateways.

### Maintainability

What happens if a policy changes? 

If the rules for when a user can view a document change, but the data required to make a decision does not, the change is very straightforward in both.

If the data that's required changes, then Cedar requires you to change the SQL statements. That implies **your access control code is coupled with your application query logic**.

In OpenFGA, if the data is already stored in the OpenFGA database, no changes are needed. If the data is not, you will need to implement a way to synchronize that data ot the OpenFGA database.

### Operations

Running OpenFGA requires operating a cluster of nodes and a database. It will become a crucial component of your application infrastucture that can't fail. 

Cedar does not require additional infrastucture. However, the database load will be higher, as applications need to retrieve data from transactional databse to inform authorization decision.


# Running the Cedar and OpenFGA examples


# Running the Cedar and OpenFGA examples
