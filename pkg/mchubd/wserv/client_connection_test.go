package wserv

import (
	"testing"
)

func TestHandleTransferInit(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *ClientConnection
		wantErr bool
	}{
		{
			name: "successful transfer init",
			setup: func() *ClientConnection {
				cc := &ClientConnection{}
				return cc
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := tt.setup()
			msg := Message{}
			cc.handleTransferInit(msg)
		})
	}
}
