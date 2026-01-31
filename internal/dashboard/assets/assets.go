package assets

import (
	"embed"
	"io/fs"
)

//go:embed all:static
//go:embed all:dist
var DashboardFS embed.FS

// GetStatic returns the filesystem for the dashboard.
// It prefers the React build (dist/) over the legacy static files.
func GetStatic() (fs.FS, error) {
	// Try React build first
	if dist, err := fs.Sub(DashboardFS, "dist"); err == nil {
		if _, err := dist.Open("index.html"); err == nil {
			return dist, nil
		}
	}
	// Fall back to legacy static folder
	return fs.Sub(DashboardFS, "static")
}

// GetLegacyStatic returns the legacy static folder filesystem.
// Used by the /static/* handler for backward compatibility.
func GetLegacyStatic() (fs.FS, error) {
	return fs.Sub(DashboardFS, "static")
}
