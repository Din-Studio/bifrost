// Package casdoor will hold the Casdoor directory adapter for the OSS identity
// layer (framework/identity): native-API client, webhook intake, incremental
// scan and full-inventory reconciliation.
//
// Phase 1 of the OSS identity work ships only this package declaration and the
// sanitized Casdoor v4.1.0 fixtures under testdata/, which pin the wire shapes
// the adapter must handle (see testdata/README.md and
// docs/architecture/identity/casdoor-bridge.mdx). The adapter itself lands in
// Phase 2/4.
package casdoor
