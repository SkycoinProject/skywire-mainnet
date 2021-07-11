//+build linux

package skyenv

const (
	packageSkywirePath = "/opt/skywire"
)

// PackageSkywirePath gets Skywire installation folder.
func PackageSkywirePath() string {
	return packageSkywirePath
}
