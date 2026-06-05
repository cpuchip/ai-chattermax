package static

import "embed"

// Migrations holds the SQL migration files, applied in lexical order by the
// boot-time runner (internal/db.Migrate). Kept in the root package alongside
// the frontend embed so the db package stays generic.
//
//go:embed migrations/*.sql
var Migrations embed.FS
