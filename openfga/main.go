package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/openfga/go-sdk/client"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatal("Usage: ./openfga-check <userID> <documentID>")
	}
	userID, documentID := os.Args[1], os.Args[2]

	// Create OpenFGA client
	fgaClient, err := client.NewSdkClient(&client.ClientConfiguration{
		ApiUrl: "http://localhost:8080", // OpenFGA server URL
	})
	if err != nil {
		log.Fatal("Failed to create OpenFGA client:", err)
	}

	// Get the store ID (in production, you'd have this configured)
	storeID := os.Getenv("OPENFGA_STORE_ID")
	if storeID == "" {
		// For demo purposes, we'll try to find/create a store
		stores, err := fgaClient.ListStores(context.Background()).Execute()
		if err != nil {
			log.Fatal("Failed to list stores:", err)
		}
		
		if len(stores.Stores) == 0 {
			log.Fatal("No OpenFGA store found. Please create a store and set OPENFGA_STORE_ID environment variable.")
		}
		
		storeID = stores.Stores[0].Id
		fmt.Printf("Using store: %s\n", storeID)
	}

	// Set the store ID
	fgaClient.SetStoreId(storeID)

	// Get the authorization model ID
	models, err := fgaClient.ReadAuthorizationModels(context.Background()).Execute()
	if err != nil {
		log.Fatal("Failed to read authorization models:", err)
	}
	
	if len(models.AuthorizationModels) == 0 {
		log.Fatal("No authorization model found. Please upload the document-management.fga model.")
	}
	
	modelID := models.AuthorizationModels[0].Id
	fgaClient.SetAuthorizationModelId(modelID)

	// Perform authorization check
	allowed, err := checkAuthorization(fgaClient, userID, documentID)
	if err != nil {
		log.Fatal("Authorization check failed:", err)
	}

	// Print result
	if allowed {
		fmt.Printf("✅ ALLOWED: %s can view %s\n", userID, documentID)
	} else {
		fmt.Printf("❌ DENIED: %s cannot view %s\n", userID, documentID)
	}
}

// checkAuthorization performs OpenFGA authorization check
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
