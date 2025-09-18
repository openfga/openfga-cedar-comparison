#!/bin/bash

# Cedar Authorization Example Setup Script

echo "🚀 Setting up Cedar Authorization Example..."

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

echo "📦 Starting PostgreSQL with test data..."
docker-compose up -d

echo "⏳ Waiting for PostgreSQL to be ready..."
sleep 5

# Check if PostgreSQL is ready
until docker-compose exec postgres pg_isready -U postgres &>/dev/null; do
    echo "   Still waiting for PostgreSQL..."
    sleep 2
done

echo "✅ PostgreSQL is ready!"

echo "📥 Installing Go dependencies..."
go mod tidy

echo "🔨 Building the application..."
go build -o cedar-check main.go

echo "✅ Setup complete!"
echo ""
echo "🧪 Try these test commands:"
echo "   ./cedar-check alice doc1    # ✅ Owner access"
echo "   ./cedar-check charlie doc2  # ✅ Organization + folder permission"  
echo "   ./cedar-check david doc1    # ❌ Cross-organization denied"
echo "   ./cedar-check bob doc4      # ✅ Explicit editor permission"
echo ""
echo "📖 See README.md for more details"
echo "🧹 Run 'docker compose down -v' when done to cleanup"
