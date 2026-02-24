package schema

import _ "embed"

// InitSQL contains the initial database schema.
//
//go:embed 001_init.sql
var InitSQL string
