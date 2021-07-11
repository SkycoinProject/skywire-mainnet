//+build darwin

package skyenv

import "os"

const (
	packageSkywirePath = "/Skywire"
)

// PackageSkywirePath gets Skywire installation folder.
func PackageSkywirePath() string {
	return os.Getenv("HOME") + packageSkywirePath
}
