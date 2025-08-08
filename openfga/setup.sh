#!/bin/bash

# OpenFGA Authorization Example Setup Script

echo "🚀 Setting up OpenFGA Authorization Example..."

# Check if docker-compose is available
if ! command -v docker-compose &> /dev/null; then
    echo "❌ docker-compose is required but not installed. Please install Docker Desktop."
    exit 1
fi

# Check if Go is available
if ! command -v go &> /dev/null; then
    echo "❌ Go is required but not installed. Please install Go 1.19+."
    exit 1
fi

# Check if curl is available
if ! command -v curl &> /dev/null; then
    echo "❌ curl is required but not installed."
    exit 1
fi

echo "📦 Starting OpenFGA and PostgreSQL..."
docker-compose up -d

echo "⏳ Waiting for OpenFGA to be ready..."
sleep 10

# Wait for OpenFGA to be healthy
until curl -f http://localhost:8080/healthz &>/dev/null; do
    echo "   Still waiting for OpenFGA..."
    sleep 3
done

echo "✅ OpenFGA is ready!"

echo "🏪 Creating OpenFGA store..."
STORE_RESPONSE=$(curl -s -X POST http://localhost:8080/stores \
  -H "Content-Type: application/json" \
  -d '{"name": "document-management"}')

STORE_ID=$(echo $STORE_RESPONSE | grep -o '"id":"[^"]*"' | cut -d'"' -f4)

if [ -z "$STORE_ID" ]; then
    echo "❌ Failed to create store. Response: $STORE_RESPONSE"
    exit 1
fi

echo "✅ Created store with ID: $STORE_ID"

echo "📋 Uploading authorization model..."
MODEL_RESPONSE=$(curl -s -X POST "http://localhost:8080/stores/$STORE_ID/authorization-models" \
  -H "Content-Type: application/json" \
  -d @document-management.json)

MODEL_ID=$(echo $MODEL_RESPONSE | grep -o '"authorization_model_id":"[^"]*"' | cut -d'"' -f4)

if [ -z "$MODEL_ID" ]; then
    echo "❌ Failed to create authorization model. Response: $MODEL_RESPONSE"
    exit 1
fi

echo "✅ Created authorization model with ID: $MODEL_ID"

echo "📝 Writing tuples..."
curl -s -X POST "http://localhost:8080/stores/$STORE_ID/write" \
  -H "Content-Type: application/json" \
  -d @- << 'EOF'
{
  "writes": {
    "tuple_keys": [
      {"user": "user:alice", "relation": "member", "object": "organization:org1"},
      {"user": "user:bob", "relation": "member", "object": "organization:org1"},
      {"user": "user:charlie", "relation": "member", "object": "organization:org1"},
      {"user": "user:david", "relation": "member", "object": "organization:org2"},
      {"user": "user:eve", "relation": "member", "object": "organization:org2"},
      
      {"user": "organization:org1", "relation": "organization", "object": "folder:folder1"},
      {"user": "user:alice", "relation": "owner", "object": "folder:folder1"},
      {"user": "organization:org2", "relation": "organization", "object": "folder:folder2"},
      {"user": "user:david", "relation": "owner", "object": "folder:folder2"},
      
      {"user": "organization:org1", "relation": "organization", "object": "document:doc1"},
      {"user": "user:alice", "relation": "owner", "object": "document:doc1"},
      {"user": "folder:folder1", "relation": "parent_folder", "object": "document:doc1"},
      
      {"user": "organization:org1", "relation": "organization", "object": "document:doc2"},
      {"user": "user:bob", "relation": "owner", "object": "document:doc2"},
      {"user": "folder:folder1", "relation": "parent_folder", "object": "document:doc2"},
      
      {"user": "organization:org2", "relation": "organization", "object": "document:doc3"},
      {"user": "user:david", "relation": "owner", "object": "document:doc3"},
      {"user": "folder:folder2", "relation": "parent_folder", "object": "document:doc3"},
      
      {"user": "organization:org1", "relation": "organization", "object": "document:doc4"},
      {"user": "user:alice", "relation": "owner", "object": "document:doc4"},
      
      {"user": "user:charlie", "relation": "viewer", "object": "document:doc2"},
      {"user": "user:bob", "relation": "editor", "object": "document:doc4"},
      {"user": "user:charlie", "relation": "viewer", "object": "document:doc4"},
      
      {"user": "user:bob", "relation": "viewer", "object": "folder:folder1"},
      {"user": "user:eve", "relation": "editor", "object": "folder:folder2"}
    ]
  }
}
EOF

echo "✅ Tuples written successfully!"

echo "📥 Installing Go dependencies..."
go mod tidy

echo "🔨 Building the application..."
go build -o openfga-check main.go

# Export environment variables for the application
export OPENFGA_STORE_ID=$STORE_ID
echo "export OPENFGA_STORE_ID=$STORE_ID" > .env

echo "✅ Setup complete!"
echo ""
echo "🧪 Try these test commands:"
echo "   export OPENFGA_STORE_ID=$STORE_ID"
echo "   ./openfga-check alice doc1    # ✅ Owner access"
echo "   ./openfga-check charlie doc2  # ✅ Organization + explicit permission"  
echo "   ./openfga-check david doc1    # ❌ Cross-organization denied"
echo "   ./openfga-check bob doc4      # ✅ Explicit editor permission"
echo ""
echo "🌐 OpenFGA Playground: http://localhost:3001"
echo "📖 See README.md for more details"
echo "🧹 Run 'docker-compose down -v' when done to cleanup"
