package mctus2

import (
	"encoding/json"
	"net/http"
)

type ProgressController struct {
	uploadProgressCache *UploadProgressCache
}

func NewProgressController(uploadProgressCache *UploadProgressCache) *ProgressController {
	return &ProgressController{uploadProgressCache: uploadProgressCache}
}

func (c *ProgressController) GetUploadProgressHandler(w http.ResponseWriter, r *http.Request) {
	uploadID := r.URL.Query().Get("uploadID")
	if uploadID == "" {
		http.Error(w, "uploadID parameter is required", http.StatusBadRequest)
		return
	}

	progress, err := c.uploadProgressCache.GetUploadProgress(uploadID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int64{"progress": progress})
}
