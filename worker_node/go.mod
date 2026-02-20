module github.com/cypher0n3/cynodeai/worker_node

go 1.25

toolchain go1.25.7

require (
	github.com/cucumber/godog v0.15.1
	github.com/cypher0n3/cynodeai/go_shared_libs v0.0.0
)

require (
	github.com/cucumber/gherkin/go/v26 v26.2.0 // indirect
	github.com/cucumber/messages/go/v21 v21.0.1 // indirect
	github.com/gofrs/uuid v4.3.1+incompatible // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-memdb v1.3.4 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/spf13/pflag v1.0.7 // indirect
)

replace github.com/cypher0n3/cynodeai/go_shared_libs => ../go_shared_libs
