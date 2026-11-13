set windows-shell := ["pwsh.exe", "-NoLogo", "-NoProfile", "-Command"]

# Build the executable into bin/
build:
    go build -o bin/munitorum.exe ./cmd/munitorum

# Build and run; the database is bin/munitorum.db
run: build
    ./bin/munitorum.exe

# Run against another database file
run-db path: build
    ./bin/munitorum.exe --db "{{path}}"

# All tests, including property tests
test:
    go test ./...

# Static checks
vet:
    go vet ./...

# Coverage report at artifacts/coverage/index.html (diagnostic, not a gate)
test-coverage:
    New-Item -ItemType Directory -Force artifacts/coverage | Out-Null
    go test ./... "-coverprofile=artifacts/coverage/coverage.out"
    go tool cover "-html=artifacts/coverage/coverage.out" -o artifacts/coverage/index.html
