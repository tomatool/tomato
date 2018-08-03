package handler

import (
	"github.com/alileza/tomato/resource/filestore"
)

func (h *Handler) downloadFileFromTheFolderWithTheFileNameAndSaveAs(name, folder, file, outputName string) error {
	r, err := h.resource.Get(name)
	if err != nil {
		return err
	}
	filestoreClient := filestore.Cast(r)

	return filestoreClient.Download(folder, file, outputName)
}
