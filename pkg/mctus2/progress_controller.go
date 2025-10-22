package mctus2

type ProgressController struct {
	UploadProgressCache *UploadProgressCache
}

func (pc *ProgressController) GetUploadProgress(uploadID string) (int64, error) {
	return pc.UploadProgressCache.GetUploadProgress(uploadID), nil
}
