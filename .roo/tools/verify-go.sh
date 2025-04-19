#!/bin/bash

set -e

# Format Source Code
go fmt ./...

# Run all tests
GIN_MODE=release go test ./...