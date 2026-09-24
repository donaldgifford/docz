// This module holds no Go code. It exists only to fence ui/ off from the
// root module (DESIGN-0017 §3): the npm tree under ui/node_modules/ ships
// stray .go files (flatted's, for one), and without a go.mod here the
// root's `go list ./...`, `go test ./...`, and golangci-lint walk into them.
// A directory with its own go.mod is a separate module and `./...` stops at
// its boundary.
module github.com/donaldgifford/docz/v2/ui

go 1.26.5
