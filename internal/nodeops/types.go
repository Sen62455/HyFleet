package nodeops

import (
	"time"

	"github.com/hyfleet/hyfleet/internal/protocol"
)

type HelperRequest struct {
	Operation protocol.NodeOperation `json:"operation"`
}

type HelperResponse struct {
	Sequence     int64            `json:"sequence"`
	Status       string           `json:"status"`
	Output       string           `json:"output,omitempty"`
	ErrorCode    string           `json:"error_code,omitempty"`
	ErrorMessage string           `json:"error_message,omitempty"`
	RolledBack   bool             `json:"rolled_back"`
	Backup       *protocol.Backup `json:"backup,omitempty"`
	CompletedAt  time.Time        `json:"completed_at"`
}

func (response HelperResponse) ProtocolResult() protocol.OperationResultRequest {
	return protocol.OperationResultRequest{
		Sequence: response.Sequence, Status: response.Status,
		Output: response.Output, ErrorCode: response.ErrorCode,
		ErrorMessage: response.ErrorMessage, RolledBack: response.RolledBack,
		Backup: response.Backup, CompletedAt: response.CompletedAt,
	}
}
