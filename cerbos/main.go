package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
)

const cerbosCheckEndpoint = "http://localhost:3592/api/check/resources"

type principal struct {
	ID    string         `json:"id"`
	Roles []string       `json:"roles"`
	Attr  map[string]any `json:"attr"`
}

type resource struct {
	Kind          string         `json:"kind"`
	PolicyVersion string         `json:"policyVersion,omitempty"`
	ID            string         `json:"id"`
	Attr          map[string]any `json:"attr"`
}

type resourceActions struct {
	Resource resource `json:"resource"`
	Actions  []string `json:"actions"`
}

type checkRequest struct {
	RequestID string            `json:"requestId"`
	Principal principal         `json:"principal"`
	Resources []resourceActions `json:"resources"`
}

type checkResult struct {
	Resource resource          `json:"resource"`
	Actions  map[string]string `json:"actions"`
}

type checkResponse struct {
	Results []checkResult `json:"results"`
}

// EntityData holds all the data needed to build Cerbos attributes
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

// queryEntityData retrieves all entity data needed for authorization
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

	if data.DocumentID == "" {
		return nil, fmt.Errorf("document %s not found", documentID)
	}

	return data, nil
}

func buildResourceAttributes(data *EntityData) map[string]any {
	resourceAttrs := map[string]any{
		"organization": data.DocumentOrg,
		"editors":      data.DocumentPermissions["editor"],
		"viewers":      data.DocumentPermissions["viewer"],
	}

	if data.DocumentOwner != nil {
		resourceAttrs["owner"] = *data.DocumentOwner
	}

	if data.FolderID != nil {
		parent := map[string]any{
			"id":           *data.FolderID,
			"organization": data.FolderOrg,
			"owner":        data.FolderOwner,
			"editors":      data.FolderPermissions["editor"],
			"viewers":      data.FolderPermissions["viewer"],
		}
		resourceAttrs["parent_folder"] = parent
	}

	return resourceAttrs
}

func buildCheckRequest(userID string, data *EntityData) (*checkRequest, error) {
	if data.DocumentID == "" {
		return nil, fmt.Errorf("document not found")
	}

	req := &checkRequest{
		RequestID: fmt.Sprintf("%s-%s-view", userID, data.DocumentID),
		Principal: principal{
			ID:    userID,
			Roles: []string{"user"},
			Attr: map[string]any{
				"organization": data.UserOrganization,
			},
		},
		Resources: []resourceActions{
			{
				Resource: resource{
					Kind:          "document",
					PolicyVersion: "default",
					ID:            data.DocumentID,
					Attr:          buildResourceAttributes(data),
				},
				Actions: []string{"ViewDocument"},
			},
		},
	}

	return req, nil
}

func checkDocument(db *sql.DB, userID, documentID string) (bool, error) {
	data, err := queryEntityData(db, userID, documentID)
	if err != nil {
		return false, err
	}

	reqPayload, err := buildCheckRequest(userID, data)
	if err != nil {
		return false, err
	}

	body, err := json.Marshal(reqPayload)
	if err != nil {
		return false, fmt.Errorf("failed to encode request: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(cerbosCheckEndpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("failed to reach Cerbos PDP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(resp.Body)
		return false, fmt.Errorf("unexpected status from Cerbos: %s - %s", resp.Status, buf.String())
	}

	var result checkResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("failed to decode Cerbos response: %w", err)
	}

	if len(result.Results) == 0 {
		return false, fmt.Errorf("empty response from Cerbos PDP")
	}

	actions := result.Results[0].Actions
	viewDecision, ok := actions["ViewDocument"]
	if !ok {
		return false, fmt.Errorf("ViewDocument result missing in response")
	}

	return viewDecision == "EFFECT_ALLOW", nil
}

func main() {
	if len(os.Args) < 3 {
		log.Fatal("Usage: ./cerbos-check <userID> <documentID>")
	}

	userID := os.Args[1]
	documentID := os.Args[2]

	db, err := sql.Open("postgres", "user=postgres password=password host=127.0.0.1 port=5432 dbname=cedar sslmode=disable")
	if err != nil {
		log.Fatal("DB connection failed:", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("DB ping failed:", err)
	}

	allowed, err := checkDocument(db, userID, documentID)
	if err != nil {
		log.Fatalf("authorization failed: %v", err)
	}

	if allowed {
		fmt.Printf("✅ ALLOWED: %s can view %s\n", userID, documentID)
	} else {
		fmt.Printf("❌ DENIED: %s cannot view %s\n", userID, documentID)
	}
}
