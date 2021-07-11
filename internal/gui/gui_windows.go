//+build windows,systray

package gui

import (
	"os"
	"path/filepath"

	"github.com/skycoin/skywire/pkg/util/osutil"
)

// TODO (darkrengarius): change path
const iconPath = "%LOCALDATA\\skywire\\icon.png"

func platformExecUninstall() error {
	localDataPath := os.Getenv("LOCALDATA")
	deinstallerPath = filepath.Join(localDataPath, "skywire", "deinstaller.ps1")
	return osutil.Run("pwsh", "-c", deinstallerPath)
}
