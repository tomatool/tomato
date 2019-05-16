package stub

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// Retrieve builds a list of files and their contents based on a passed filepath
// the stubs are keyed by their filename
func Retrieve(stubsPath string) (map[string][]byte, error) {
	// add the stubs available to the resources
	var stubs map[string][]byte
	if stubsPath != "" {
		// recurse and get file names
		err := filepath.Walk(stubsPath,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				// read and add the files to our stubs
				data, err := ioutil.ReadFile(info.Name())
				if err != nil {
					return err
				}
				if _, ok := stubs[info.Name()]; ok {
					return errors.Errorf("file name not unique, please use unique file names: %s", info.Name())
				}
				stubs[info.Name()] = data
				return nil
			})
		if err != nil {
			return nil, err
		}
	}
	return stubs, nil
}
