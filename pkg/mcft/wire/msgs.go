package wire

type InitUploadMsg struct {
	MsgType   string `json:"msg_type"`
	APIToken  string `json:"api_token"`
	ProjectID int    `json:"project_id"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Checksum  string `json:"checksum,omitempty"`
	UploadID  string `json:"upload_id,omitempty"`
}

type InitUploadAckMsg struct {
	MsgType    string `json:"msg_type"`
	UploadID   string `json:"upload_id"`
	NextOffset int64  `json:"next_offset"`
}

type UploadChunkMsg struct {
	MsgType       string `json:"msg_type"`
	UploadID      string `json:"upload_id"`
	Offset        int64  `json:"offset"`
	Size          int64  `json:"size"`
	BlockChecksum string `json:"block_checksum,omitempty"`
}

type UploadChunkAckMsg struct {
	MsgType    string `json:"msg_type"`
	UploadID   string `json:"upload_id"`
	NextOffset int64  `json:"next_offset"`
}

type FinalizeUploadMsg struct {
	MsgType  string `json:"msg_type"`
	UploadID string `json:"upload_id"`
}

type FinalizeUploadAckMsg struct {
	MsgType  string `json:"msg_type"`
	UploadID string `json:"upload_id"`
	Status   string `json:"status"`
	Message  string `json:"message"`
}

type ErrorMsg struct {
	MsgType string `json:"msg_type"`
	Message string `json:"message"`
	ID      string `json:"id"`
}
