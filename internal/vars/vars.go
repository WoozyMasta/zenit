// Package vars holds build-time variables populated via the linker (ldflags).
package vars

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// License of the project
const License = "AGPL-3.0"

var (
	// Name of the project
	Name = "Zenit"

	// Version of application (git tag) semver/tag, e.g. v1.2.3
	Version = "dev"

	// Commit is the current git commit, full or short git SHA
	Commit = "unknown"

	// Revision build, count of commits
	Revision = 0

	// BuildTime is the time of start build app, RFC3339 UTC
	BuildTime = time.Unix(0, 0)

	// URL to repository (https)
	URL = "https://github.com/woozymasta/zenit"

	_revision  string
	_buildTime string
)

// BuildInfo optional helper to expose safe values everywhere.
type BuildInfo struct {
	// betteralign:ignore

	// Project name
	Name string `json:"name" example:"Zenit"`

	// Version of application (git tag) semver/tag, e.g. v1.2.3
	Version string `json:"version" example:"v1.2.3"`

	// Current git commit, full or short git SHA
	Commit string `json:"commit" example:"da15c174cd2ada1ad247906536c101e8f6799def"`

	// Current git commit short SHA
	CommitShort string `json:"commit_short,omitempty" example:"da15c17"`

	// Revision build, count of commits
	Revision int `json:"revision,omitempty" example:"1337"`

	// Time of start build app, RFC3339 UTC
	BuildTime time.Time `json:"build_time,omitempty" example:"1970-01-01T00:00:00Z"`

	// URL to repository (https)
	URL string `json:"url,omitempty" example:"https://github.com/woozymasta/zenit"`

	// License
	License string `json:"license,omitempty" example:"AGPL-3.0"`
}

func init() {
	if n, err := strconv.Atoi(_revision); err == nil {
		Revision = n
	}

	if _buildTime != "" {
		if t, err := time.Parse(time.RFC3339, _buildTime); err == nil {
			BuildTime = t.UTC()
		}
	}
}

// Print writes the build information to the standard output.
func Print() {
	fmt.Printf(`name:     %s
url:      %s
file:     %s
version:  %s
commit:   %s
revision: %d
built:    %s
license:  %s
`, Name, URL, os.Args[0], Version, Commit, Revision, BuildTime, License)
}

// Info returns a BuildInfo struct containing detailed build metadata.
func Info() BuildInfo {
	return BuildInfo{
		Name:        Name,
		Version:     Version,
		Commit:      Commit,
		CommitShort: CommitShort(),
		Revision:    Revision,
		BuildTime:   BuildTime,
		URL:         URL,
		License:     License,
	}
}

// Ver returns a simplified BuildInfo struct containing versioning details.
func Ver() BuildInfo {
	return BuildInfo{
		Name:     Name,
		Version:  Version,
		Commit:   Commit,
		Revision: Revision,
	}
}

// CommitShort returns the first 7 characters of the git commit hash.
func CommitShort() string {
	if len(Commit) > 7 {
		return Commit[:7]
	}

	return Commit
}
