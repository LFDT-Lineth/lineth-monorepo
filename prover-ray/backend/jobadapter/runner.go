// Package jobadapter turns coordinator proof requests into backend jobs.
//
// Runner is the shared request-to-proof path used by protocol adapters such as
// filesystem and a future prover-side gateway adapter. It decodes a
// getZkL2ExecutionProofV1 request, SSZ-encodes the payload for the RISC-V guest,
// calls the Prover, and formats the V1 response body.
package jobadapter

import (
	"context"
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
)

// Prover is the proving engine: it receives a backend.Job and returns the proof
// result. backend.Core satisfies this role; tests use a mock so they never need
// to build a circuit.
type Prover interface {
	Prove(ctx context.Context, job backend.Job) backend.Result
}

// Runner owns the request-to-proof flow for getZkL2ExecutionProofV1: decode
// the coordinator request body, build a backend.Job, call the prover, and
// format the response.
type Runner struct {
	prover        Prover
	proverVersion string
}

// RunStatus is the normalized outcome status for one request body.
type RunStatus string

const (
	RunStatusSuccess RunStatus = "success"
	RunStatusFailed  RunStatus = "failed"
)

// FailureCode classifies failed outcomes in the same style as the future
// gateway result flow. Filesystem responses are still provisional, but keeping
// a code here avoids reducing failures to a bool.
type FailureCode string

const (
	FailureCodeOOM           FailureCode = "oom"
	FailureCodeInternalError FailureCode = "internal_error"
	FailureCodeInvalidInput  FailureCode = "invalid_input"
)

// RunResult is the outcome for one request body. Callers write ResponseBody
// back to their queue and use Status/FailureCode for protocol-specific result
// handling such as archive suffixes or future gateway result submission.
type RunResult struct {
	ResponseBody any
	Status       RunStatus
	FailureCode  FailureCode
	Err          error
}

// NewRunner creates the reusable request-to-proof runner.
func NewRunner(prover Prover, proverVersion string) (*Runner, error) {
	if prover == nil {
		return nil, fmt.Errorf("jobadapter.NewRunner: prover must not be nil")
	}
	if proverVersion == "" {
		return nil, fmt.Errorf("jobadapter.NewRunner: proverVersion must be set")
	}
	return &Runner{prover: prover, proverVersion: proverVersion}, nil
}

// Run runs one raw request body. Decode, validation, and proof failures all map
// to a response body rather than a returned error.
func (r *Runner) Run(ctx context.Context, id string, data []byte) RunResult {
	req, err := DecodeRequest(data)
	if err != nil {
		return failedRunResult(id, FailureCodeInvalidInput, err)
	}
	if len(req.Payloads) != 1 {
		return failedRunResult(id, FailureCodeInvalidInput, fmt.Errorf(
			"multi-block requests are not supported (got %d payloads): %w",
			len(req.Payloads), backend.ErrNotImplemented))
	}

	payload := req.Payloads[0]
	if len(payload.ForcedTransactions) != 0 {
		return failedRunResult(id, FailureCodeInvalidInput, fmt.Errorf(
			"forced transactions are not supported (got %d): %w",
			len(payload.ForcedTransactions), backend.ErrNotImplemented))
	}

	result := r.prover.Prove(ctx, backend.Job{
		ID:         id,
		Type:       backend.ProofTypeL2Execution,
		StartBlock: payload.BlockNumber,
		EndBlock:   payload.BlockNumber,
		Payload:    payload.FramedSSZ,
	})
	if result.Status != backend.ResultStatusOK {
		err := result.Err
		if err == nil {
			err = fmt.Errorf("prover returned status %s", result.Status)
		}
		return failedRunResult(id, FailureCodeInternalError, err)
	}
	return RunResult{
		ResponseBody: newExecutionResponse(result, payload.BlockNumber, r.proverVersion),
		Status:       RunStatusSuccess,
	}
}

func failedRunResult(id string, code FailureCode, err error) RunResult {
	return RunResult{
		ResponseBody: failureResponse(id, code, err),
		Status:       RunStatusFailed,
		FailureCode:  code,
		Err:          err,
	}
}
