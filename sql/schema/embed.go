package schema

import _ "embed"

// InitSQL contains the initial database schema.
//
//go:embed 001_init.sql
var InitSQL string

// LtreeSQL adds the ltree path column to releases.
//
//go:embed 002_ltree.sql
var LtreeSQL string
