#!/bin/bash
# Code quality script for new-api project

echo "Running Go code quality checks..."

echo "1. Running go vet..."
go vet ./...

echo "2. Running go fmt check..."
if [ -n "$(gofmt -l .)" ]; then
    echo "Go fmt issues found:"
    gofmt -l .
    echo "Run 'gofmt -w .' to fix formatting"
    exit 1
else
    echo "Go fmt: OK"
fi

echo "3. Running tests..."
go test -v ./... -race

echo "4. Running tests with coverage..."
go test -v ./... -race -coverprofile=coverage.out
go tool cover -func=coverage.out

echo "All quality checks completed!"