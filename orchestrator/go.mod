module github.com/cypher0n3/cynodeai/orchestrator

go 1.25

toolchain go1.25.7

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/cypher0n3/cynodeai/go_shared_libs v0.0.0
	github.com/golang-jwt/jwt/v5 v5.2.2
	github.com/google/uuid v1.6.0
	github.com/lib/pq v1.10.9
	golang.org/x/crypto v0.38.0
)

require golang.org/x/sys v0.33.0 // indirect

replace github.com/cypher0n3/cynodeai/go_shared_libs => ../go_shared_libs
