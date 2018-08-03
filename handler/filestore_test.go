package handler

import "testing"

var (
	resourceFilestoreClient = &resourceFilestoreClientMock{}
)

type resourceFilestoreClientMock struct{}

func (c *resourceFilestoreClientMock) Download(folder, file, outputFile string) error {
	return nil
}

func (c *resourceFilestoreClientMock) Ready() error               { return nil }
func (c *resourceFilestoreClientMock) Close() error               { return nil }
func (c *resourceFilestoreClientMock) List() (interface{}, error) { return nil, nil }
func (c *resourceFilestoreClientMock) Upload(target string, payload []byte) (int, error) {
	return 5000, nil
}
func (c *resourceFilestoreClientMock) Delete(target string) error { return nil }

func TestDownloadFileFromTheFolderWithTheFileNameAndSaveAs(t *testing.T) {
	if err := h.downloadFileFromTheFolderWithTheFileNameAndSaveAs("filestore-resource", "hdhuwk", "input.csv", "output.csv"); err != nil {
		t.Error(err)
	}
}
