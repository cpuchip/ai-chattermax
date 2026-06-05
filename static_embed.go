package static

import (
	"embed"
	"io/fs"
)

//go:embed all:frontend/dist
var embedded embed.FS

func FS() (fs.FS, error) {
	return fs.Sub(embedded, "frontend/dist")
}
