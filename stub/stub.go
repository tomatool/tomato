package stub

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

type Stubs struct {
	stubs map[string][]byte
}

// Get retrieves the stub content based on the keyed name
func (s *Stubs) Get(identifier string) ([]byte, error) {
	stub, ok := s.stubs[identifier]
	if !ok {
		identifiers := make([]string, len(s.stubs))
		for id := range s.stubs {
			identifiers = append(identifiers, id)
		}
		return nil, errors.Errorf("no stubs loaded with name: %s available: %s", identifier, strings.Join(identifiers, ", "))
	}

	return stub, nil
}

// RetrieveFiles builds a list of files and their contents based on a passed filepath
// the stubs are keyed by their filename
func RetrieveFiles(stubsPath string) (*Stubs, error) {
	// add the stubs available to the resources
	s := Stubs{stubs: make(map[string][]byte)}
	if stubsPath != "" {
		// recurse and get file names
		err := filepath.Walk(stubsPath,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if info.IsDir() {
					return nil
				}
				// read and add the files to our stubs
				data, err := ioutil.ReadFile(path)
				if err != nil {
					return err
				}
				if _, ok := s.stubs[info.Name()]; ok {
					return errors.Errorf("file name not unique, please use unique file names: %s", info.Name())
				}
				s.stubs[info.Name()] = data
				return nil
			})
		if err != nil {
			return nil, err
		}
	}
	return &s, nil
}
