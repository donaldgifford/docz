// doczconsumer is a standalone module that imports the public pkg/doczcore
// surface exactly as an external consumer (docz-api) would, proving the
// promoted packages are importable and usable from outside the docz module
// (DESIGN-0007 / IMPL-0013 Phase 3). It uses a local replace while docz is
// unreleased; a real consumer pins a published tag instead.
module doczconsumer

go 1.26.4

require github.com/donaldgifford/docz v0.0.0

require (
	github.com/kr/text v0.2.0 // indirect
	github.com/rogpeppe/go-internal v1.9.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
)

replace github.com/donaldgifford/docz => ../..
