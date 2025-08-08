package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/cedar-policy/cedar-go"
	_ "github.com/lib/pq"
)

// Note: This example demonstrates schema usage concepts.
// The exact Cedar Go SDK API may vary - check current documentation.
//
// Key benefits of using schema.cedarschema:
// 1. Type safety - validates entity types and attributes
// 2. Policy validation - ensures policies match schema
// 3. IDE support - enables autocompletion and error checking
// 4. Documentation - serves as a contract for the authorization model

// EntityData holds all the data needed to build Cedar entities
type EntityData struct {
	UserOrganization    string
	DocumentID          string
	DocumentOrg         string
	FolderID            *string
	DocumentOwner       *string
	FolderOrg           *string
	FolderOwner         *string
	DocumentPermissions map[string][]string // permissionType -> userIDs
	FolderPermissions   map[string][]string // permissionType -> userIDs
}

// queryEntityData retrieves all entity data needed for Cedar authorization
func queryEntityData(db *sql.DB, userID, documentID string) (*EntityData, error) {
	query := `
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
		di.doc_id,
		di.doc_org_id,
		di.folder_id,
		di.doc_owner_id,
		di.folder_org_id,
		di.folder_owner_id,
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
	`

	rows, err := db.Query(query, userID, documentID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	data := &EntityData{
		DocumentPermissions: make(map[string][]string),
		FolderPermissions:   make(map[string][]string),
	}

	for rows.Next() {
		var userOrg, docID, docOrg, folderID, docOwner, folderOrg, folderOwner sql.NullString
		var permUserID, permType, resourceTypeCol string

		err := rows.Scan(&userOrg, &docID, &docOrg, &folderID, &docOwner,
			&folderOrg, &folderOwner, &permUserID, &permType, &resourceTypeCol)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		// Set basic entity data (only on first row)
		if data.DocumentID == "" {
			data.UserOrganization = userOrg.String
			data.DocumentID = docID.String
			data.DocumentOrg = docOrg.String
			if folderID.Valid {
				data.FolderID = &folderID.String
			}
			if docOwner.Valid {
				data.DocumentOwner = &docOwner.String
			}
			if folderOrg.Valid {
				data.FolderOrg = &folderOrg.String
			}
			if folderOwner.Valid {
				data.FolderOwner = &folderOwner.String
			}
		}

		// Process permissions
		if permUserID != "" && permType != "" {
			if resourceTypeCol == "document" {
				data.DocumentPermissions[permType] = append(
					data.DocumentPermissions[permType], permUserID)
			} else if resourceTypeCol == "folder" {
				data.FolderPermissions[permType] = append(
					data.FolderPermissions[permType], permUserID)
			}
		}
	}

	return data, nil
}

// checkAuthorization builds Cedar entities from data and performs authorization check
func checkAuthorization(policySet *cedar.PolicySet, data *EntityData, userID, documentID string) (bool, error) {
	// Build Cedar entities
	entities := cedar.EntityMap{}

	// User entity
	userAttrs := cedar.RecordMap{}
	if data.UserOrganization != "" {
		orgUID := cedar.NewEntityUID(cedar.EntityType("DocumentManagement::Organization"), cedar.String(data.UserOrganization))
		userAttrs["organization"] = cedar.EntityUID(orgUID)
		entities[orgUID] = cedar.Entity{
			UID:        orgUID,
			Attributes: cedar.NewRecord(cedar.RecordMap{"name": cedar.String(data.UserOrganization)}),
		}
	}
	userUID := cedar.NewEntityUID(cedar.EntityType("DocumentManagement::User"), cedar.String(userID))
	entities[userUID] = cedar.Entity{
		UID:        userUID,
		Attributes: cedar.NewRecord(userAttrs),
	}

	// Document entity
	docAttrs := cedar.RecordMap{"name": cedar.String(data.DocumentID)}
	if data.DocumentOrg != "" {
		orgUID := cedar.NewEntityUID(cedar.EntityType("DocumentManagement::Organization"), cedar.String(data.DocumentOrg))
		docAttrs["organization"] = cedar.EntityUID(orgUID)
	}
	if data.DocumentOwner != nil {
		ownerUID := cedar.NewEntityUID(cedar.EntityType("DocumentManagement::User"), cedar.String(*data.DocumentOwner))
		docAttrs["owner"] = cedar.EntityUID(ownerUID)
	}
	if data.FolderID != nil {
		folderUID := cedar.NewEntityUID(cedar.EntityType("DocumentManagement::Folder"), cedar.String(*data.FolderID))
		docAttrs["parent_folder"] = cedar.EntityUID(folderUID)
	}

	// Add document permissions (editors, viewers)
	if len(data.DocumentPermissions["editor"]) > 0 {
		var editorValues []cedar.Value
		for _, editorID := range data.DocumentPermissions["editor"] {
			editorUID := cedar.NewEntityUID(cedar.EntityType("DocumentManagement::User"), cedar.String(editorID))
			editorValues = append(editorValues, cedar.EntityUID(editorUID))
		}
		docAttrs["editors"] = cedar.NewSet(editorValues...)
	}
	if len(data.DocumentPermissions["viewer"]) > 0 {
		var viewerValues []cedar.Value
		for _, viewerID := range data.DocumentPermissions["viewer"] {
			viewerUID := cedar.NewEntityUID(cedar.EntityType("DocumentManagement::User"), cedar.String(viewerID))
			viewerValues = append(viewerValues, cedar.EntityUID(viewerUID))
		}
		docAttrs["viewers"] = cedar.NewSet(viewerValues...)
	}

	// Create folder entity if exists
	if data.FolderID != nil {
		// Add folder entity
		folderAttrs := cedar.RecordMap{"name": cedar.String(*data.FolderID)}
		if data.FolderOrg != nil {
			orgUID := cedar.NewEntityUID(cedar.EntityType("DocumentManagement::Organization"), cedar.String(*data.FolderOrg))
			folderAttrs["organization"] = cedar.EntityUID(orgUID)
		}
		if data.FolderOwner != nil {
			ownerUID := cedar.NewEntityUID(cedar.EntityType("DocumentManagement::User"), cedar.String(*data.FolderOwner))
			folderAttrs["owner"] = cedar.EntityUID(ownerUID)
		}

		// Add folder permissions (editors, viewers)
		if len(data.FolderPermissions["editor"]) > 0 {
			var editorValues []cedar.Value
			for _, editorID := range data.FolderPermissions["editor"] {
				editorUID := cedar.NewEntityUID(cedar.EntityType("DocumentManagement::User"), cedar.String(editorID))
				editorValues = append(editorValues, cedar.EntityUID(editorUID))
			}
			folderAttrs["editors"] = cedar.NewSet(editorValues...)
		}
		if len(data.FolderPermissions["viewer"]) > 0 {
			var viewerValues []cedar.Value
			for _, viewerID := range data.FolderPermissions["viewer"] {
				viewerUID := cedar.NewEntityUID(cedar.EntityType("DocumentManagement::User"), cedar.String(viewerID))
				viewerValues = append(viewerValues, cedar.EntityUID(viewerUID))
			}
			folderAttrs["viewers"] = cedar.NewSet(viewerValues...)
		}

		folderUID := cedar.NewEntityUID(cedar.EntityType("DocumentManagement::Folder"), cedar.String(*data.FolderID))
		entities[folderUID] = cedar.Entity{
			UID:        folderUID,
			Attributes: cedar.NewRecord(folderAttrs),
		}
	}
	docUID := cedar.NewEntityUID(cedar.EntityType("DocumentManagement::Document"), cedar.String(documentID))
	entities[docUID] = cedar.Entity{
		UID:        docUID,
		Attributes: cedar.NewRecord(docAttrs),
	}

	// Create authorization request
	request := cedar.Request{
		Principal: userUID,
		Action:    cedar.NewEntityUID(cedar.EntityType("DocumentManagement::Action"), cedar.String("ViewDocument")),
		Resource:  docUID,
		Context:   cedar.NewRecord(cedar.RecordMap{}),
	}

	// Authorize
	decision, diagnostic := cedar.Authorize(policySet, entities, request)
	if len(diagnostic.Errors) > 0 {
		return false, fmt.Errorf("authorization errors: %v", diagnostic.Errors)
	}

	return decision == cedar.Allow, nil
}

func main() {
	if len(os.Args) < 3 {
		log.Fatal("Usage: ./cedar-check <userID> <documentID>")
	}
	userID, documentID := os.Args[1], os.Args[2]

	// Connect to database
	db, err := sql.Open("postgres", "user=postgres password=password host=localhost port=5432 dbname=cedar sslmode=disable")
	if err != nil {
		log.Fatal("DB connection failed:", err)
	}
	defer db.Close()

	// Load Cedar policies
	policies, err := os.ReadFile("policies.cedar")
	if err != nil {
		log.Fatal("Failed to load policies:", err)
	}
	policySet, err := cedar.NewPolicySetFromBytes("policies.cedar", policies)
	if err != nil {
		log.Fatal("Failed to parse policies:", err)
	}

	// Query database for ALL entity data needed for Cedar policies
	data, err := queryEntityData(db, userID, documentID)
	if err != nil {
		log.Fatal("Failed to query entity data:", err)
	}

	// Perform authorization check
	allowed, err := checkAuthorization(policySet, data, userID, documentID)
	if err != nil {
		log.Fatal("Authorization failed:", err)
	}

	// Print result
	if allowed {
		fmt.Printf("✅ ALLOWED: %s can view %s\n", userID, documentID)
	} else {
		fmt.Printf("❌ DENIED: %s cannot view %s\n", userID, documentID)
	}
}
