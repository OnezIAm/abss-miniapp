#!/bin/bash

# Navigate to the directory where the script is located
cd "$(dirname "$0")"

# Set the port to 8585 to match the frontend configuration
export ADDR=:8585

# Ensure the app uses the local 'data' directory for the database (Portable Mode)
export APP_DATA_DIR="$PWD/data"

echo "---------------------------------------------------"
echo "Starting Bank Consolidation App..."
echo "Backend Port: $ADDR"
echo "Data Directory: $APP_DATA_DIR"
echo "---------------------------------------------------"

# Kill existing process if running
pkill -f bank-app-mac || true
sleep 1

# Run the compiled binary in background
./bank-app-mac &
BACKEND_PID=$!

# Wait a moment for the server to initialize
echo "Waiting for server to start..."
sleep 2

# Open the default web browser
echo "Opening browser to http://localhost:8585..."
open "http://localhost:8585"

# Keep the terminal window open and wait for the backend process
# Press Ctrl+C in the terminal to stop the server
wait $BACKEND_PID
