package globus

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-resty/resty/v2"
)

var ErrGlobusAPI = errors.New("globus api")

// ErrorResponse describes the JSON that Globus responds with when there is an error in an API call
type ErrorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Resource  string `json:"resource"`
}

func ToErrorFromResponse(resp *resty.Response) (*ErrorResponse, error) {
	var errorResponse ErrorResponse
	if err := json.Unmarshal(resp.Body(), &errorResponse); err != nil {
		return nil, errors.Join(ErrGlobusAPI, fmt.Errorf("(HTTP Status: %d)- unable to parse json error response: %s", resp.RawResponse.StatusCode, err))
	}

	return &errorResponse, errors.Join(ErrGlobusAPI, fmt.Errorf("(HTTP Status: %d)- %s: %s", resp.RawResponse.StatusCode, errorResponse.Code, errorResponse.Message))
}
