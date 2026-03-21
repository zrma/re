package re

import "github.com/spf13/afero"

func newOSFileSystem() afero.Fs {
	return afero.NewOsFs()
}
