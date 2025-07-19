// Package version provides the current version of the Atsuko Nexus application.
// This package allows other parts of the application to access the current release version string.
package version

// Current defines the current release version of the application.
// This value should be updated manually before each release.
var Current = "v1.3.1-alpha"

// Get returns the current version string.
// Use this function to programmatically access the application version.
func Get() string {
	return Current
}
