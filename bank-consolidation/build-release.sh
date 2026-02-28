#!/bin/bash
set -e

echo "---------------------------------------------------"
echo "Step 1: Building Frontend (Next.js export)"
echo "---------------------------------------------------"
cd frontend
# Clean previous build
rm -rf out
# Install dependencies if needed
if [ ! -d "node_modules" ]; then
    echo "Installing frontend dependencies..."
    npm install
fi
npm run build
cd ..

echo "---------------------------------------------------"
echo "Step 2: Building Backend Binaries"
echo "---------------------------------------------------"

# Build for Mac (Native)
echo "Building for Mac..."
CGO_ENABLED=0 go build -buildvcs=false -o bank-app-mac .

# Build for Windows (Cross-compile)
echo "Building for Windows..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -buildvcs=false -o bank-app-win.exe .

echo "---------------------------------------------------"
echo "Build Complete!"
echo "---------------------------------------------------"
echo "You can now distribute these files:"
echo "1. bank-app-mac      (For Mac users)"
echo "2. bank-app-win.exe  (For Windows users)"
echo ""
echo "Note: The 'data' folder containing your database"
echo "will be created automatically next to the app."
echo "---------------------------------------------------"
