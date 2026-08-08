package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/hyfleet/hyfleet/internal/cryptoutil"
	"github.com/hyfleet/hyfleet/internal/nodeops"
	"github.com/hyfleet/hyfleet/internal/protocol"
)

func (agent *Agent) runOperationCycle(ctx context.Context) error {
	if agent.localStore == nil || agent.state.NodeCredential == "" {
		return nil
	}
	if err := agent.flushOperationResults(ctx); err != nil {
		return err
	}
	after, err := agent.localStore.lastOperationSequence(ctx)
	if err != nil {
		return err
	}
	var response protocol.NodeOperationsResponse
	status, err := agent.doJSON(
		ctx, http.MethodGet, "/agent/v1/operations?after="+strconv.FormatInt(after, 10),
		nil, cryptoutil.NewID(), true, &response,
	)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("operations poll returned status %d", status)
	}
	for _, operation := range response.Operations {
		if operation.Sequence <= after || operation.ID == "" {
			return errors.New("operation sequence is invalid or out of order")
		}
		result := agent.executeNodeOperation(ctx, operation)
		result.Sequence = operation.Sequence
		result.Output = nodeops.SanitizeOutput(
			result.Output, nodeops.MaxLogLines, nodeops.MaxOutputSize,
		)
		result.ErrorMessage = nodeops.SanitizeMessage(result.ErrorMessage, 512)
		if result.CompletedAt.IsZero() {
			result.CompletedAt = time.Now().UTC()
		}
		if err := agent.localStore.recordOperationResult(
			ctx, operation, result, time.Now().UTC(),
		); err != nil {
			return err
		}
		after = operation.Sequence
		if err := agent.flushOperationResults(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (agent *Agent) flushOperationResults(ctx context.Context) error {
	results, err := agent.localStore.listPendingOperationResults(ctx, 20)
	if err != nil {
		return err
	}
	for _, pending := range results {
		status, err := agent.doJSON(
			ctx, http.MethodPost,
			"/agent/v1/operations/"+pending.OperationID+"/result",
			pending.Result, cryptoutil.NewID(), true, nil,
		)
		if err != nil {
			_ = agent.localStore.recordOperationResultFailure(
				ctx, pending.OperationID, "transport_error", time.Now().UTC(),
			)
			return err
		}
		if status != http.StatusNoContent {
			_ = agent.localStore.recordOperationResultFailure(
				ctx, pending.OperationID, "invalid_response", time.Now().UTC(),
			)
			return fmt.Errorf("operation result returned status %d", status)
		}
		if err := agent.localStore.markOperationResultReported(
			ctx, pending.OperationID, time.Now().UTC(),
		); err != nil {
			return err
		}
	}
	return nil
}

func (agent *Agent) executeNodeOperation(
	ctx context.Context,
	operation protocol.NodeOperation,
) protocol.OperationResultRequest {
	if agent.operationExecutor != nil {
		return agent.operationExecutor(ctx, operation)
	}
	return agent.executeNodeOperationWithHelper(ctx, operation)
}

func (agent *Agent) executeNodeOperationWithHelper(
	ctx context.Context,
	operation protocol.NodeOperation,
) protocol.OperationResultRequest {
	dialer := net.Dialer{Timeout: 3 * time.Second}
	connection, err := dialer.DialContext(ctx, "unix", agent.config.OperationsSocketPath)
	if err != nil {
		return failedOperationResult(operation.Sequence, "operations_helper_unavailable", err)
	}
	defer connection.Close()
	deadline := time.Now().Add(45 * time.Second)
	_ = connection.SetDeadline(deadline)
	if err := json.NewEncoder(connection).Encode(nodeops.HelperRequest{Operation: operation}); err != nil {
		return failedOperationResult(operation.Sequence, "operations_helper_write_failed", err)
	}
	var response nodeops.HelperResponse
	if err := json.NewDecoder(io.LimitReader(connection, 64*1024)).Decode(&response); err != nil {
		return failedOperationResult(operation.Sequence, "operations_helper_read_failed", err)
	}
	if response.Sequence != operation.Sequence ||
		(response.Status != "succeeded" && response.Status != "failed") ||
		response.CompletedAt.IsZero() {
		return failedOperationResult(
			operation.Sequence, "operations_helper_invalid_response",
			errors.New("helper returned invalid result fields"),
		)
	}
	return response.ProtocolResult()
}

func failedOperationResult(
	sequence int64,
	errorCode string,
	err error,
) protocol.OperationResultRequest {
	message := "operation failed"
	if err != nil {
		message = err.Error()
	}
	return protocol.OperationResultRequest{
		Sequence: sequence, Status: "failed", ErrorCode: errorCode,
		ErrorMessage: nodeops.SanitizeMessage(message, 512), CompletedAt: time.Now().UTC(),
	}
}
