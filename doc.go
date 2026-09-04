// Package datorium is a smart Go client for DatoriumDB's HTTP API v1
// (compatible with DatoriumDB v1.1.0).
//
// Import path: github.com/JohnAD/datorium-client-go/v2
//
// It caches establishment configuration, routes JSON command requests to
// the correct shard members, retries wrongMachine responses, and helps
// resolve document references.
package datorium
