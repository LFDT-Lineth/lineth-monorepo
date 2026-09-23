// Package jobadapter turns coordinator proof requests into backend jobs.
//
// Runner is the shared request-to-proof path used by protocol adapters such as
// filesystem and a future prover-side gateway adapter. It dispatches a typed
// request to the matching decoder, builds a backend.Job, calls the Prover, and
// formats the response body.
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

// Runner owns the request-to-proof flow: decode the coordinator request body,
// build a backend.Job, call the prover, and format the response.
type Runner struct {
	prover        Prover
	proverVersion string
	mode          backend.ProverMode
}

// RunnerOption configures a Runner.
type RunnerOption func(*Runner)

// WithMode sets the prover mode; the default is backend.ProverModeFull.
func WithMode(m backend.ProverMode) RunnerOption {
	return func(r *Runner) { r.mode = m }
}

// RunRequest is one raw coordinator request plus the proof type selected by
// the protocol adapter.
type RunRequest struct {
	ID   string
	Type backend.ProofType
	Body []byte
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
func NewRunner(prover Prover, proverVersion string, opts ...RunnerOption) (*Runner, error) {
	if prover == nil {
		return nil, fmt.Errorf("jobadapter.NewRunner: prover must not be nil")
	}
	if proverVersion == "" {
		return nil, fmt.Errorf("jobadapter.NewRunner: proverVersion must be set")
	}
	r := &Runner{prover: prover, proverVersion: proverVersion, mode: backend.ProverModeFull}
	for _, opt := range opts {
		opt(r)
	}
	return r, nil
}

// responseVersion suffixes the prover version with the mode for dev modes.
func (r *Runner) responseVersion() string {
	if r.mode.IsDev() {
		return r.proverVersion + "-" + string(r.mode)
	}
	return r.proverVersion
}

// Run runs one raw request. Decode, validation, and proof failures all map to a
// response body rather than a returned error.
func (r *Runner) Run(ctx context.Context, req RunRequest) RunResult {
	switch req.Type {
	case backend.ProofTypeL2Execution:
		return r.runL2Execution(ctx, req)
	case backend.ProofTypeRollup:
		return r.runRollup(ctx, req)
	case backend.ProofTypeRollupAggregation:
		return r.runAggregation(ctx, req)
	default:
		return failedRunResult(req.ID, FailureCodeInvalidInput, fmt.Errorf(
			"proof type %q is not supported: %w", req.Type, backend.ErrNotImplemented))
	}
}

func (r *Runner) runL2Execution(ctx context.Context, runReq RunRequest) RunResult {
	req, err := DecodeL2ExecutionRequest(runReq.Body)
	if err != nil {
		return failedRunResult(runReq.ID, FailureCodeInvalidInput, err)
	}

	// dev-mock runs no guest, so it accepts ranges and forced transactions. The
	// guest-running modes support a single block only, today.
	if r.mode != backend.ProverModeDevMock {
		if len(req.Payloads) != 1 {
			return failedRunResult(runReq.ID, FailureCodeInvalidInput, fmt.Errorf(
				"multi-block requests are not supported (got %d payloads): %w",
				len(req.Payloads), backend.ErrNotImplemented))
		}
		if len(req.Payloads[0].ForcedTransactions) != 0 {
			return failedRunResult(runReq.ID, FailureCodeInvalidInput, fmt.Errorf(
				"forced transactions are not supported (got %d): %w",
				len(req.Payloads[0].ForcedTransactions), backend.ErrNotImplemented))
		}
	}

	startBlock := req.Payloads[0].BlockNumber
	endBlock := req.Payloads[len(req.Payloads)-1].BlockNumber
	result := r.prover.Prove(ctx, backend.Job{
		ID:         runReq.ID,
		Type:       runReq.Type,
		StartBlock: startBlock,
		EndBlock:   endBlock,
		Payload:    req.Payloads[0].FramedSSZ,
	})
	if result.Status != backend.ResultStatusOK {
		return failedRunResult(runReq.ID, FailureCodeInternalError, proverErr(result))
	}
	return RunResult{
		ResponseBody: newExecutionResponse(result, startBlock, r.responseVersion(), req.ProgramVk),
		Status:       RunStatusSuccess,
	}
}

// runRollup decodes a rollup request, proves, and shapes the V1 response. The
// block range comes from the embedded l2-execution proofs (decoder-guaranteed
// non-empty).
func (r *Runner) runRollup(ctx context.Context, runReq RunRequest) RunResult {
	req, err := DecodeRollupRequest(runReq.Body)
	if err != nil {
		return failedRunResult(runReq.ID, FailureCodeInvalidInput, err)
	}

	startBlock := req.L2ExecutionProofs[0].StartBlockNumber
	endBlock := req.L2ExecutionProofs[len(req.L2ExecutionProofs)-1].PublicInputs.EndBlockNumber

	result := r.prover.Prove(ctx, backend.Job{
		ID:         runReq.ID,
		Type:       runReq.Type,
		StartBlock: startBlock,
		EndBlock:   endBlock,
		Payload:    nil, // recursion guest input format undecided; mock passes no payload
	})
	if result.Status != backend.ResultStatusOK {
		return failedRunResult(runReq.ID, FailureCodeInternalError, proverErr(result))
	}
	return RunResult{
		ResponseBody: newRollupResponse(result, startBlock, r.responseVersion(), req.ProgramVk),
		Status:       RunStatusSuccess,
	}
}

// runAggregation decodes an aggregation request, proves, and shapes the V1
// response. The block range comes from the embedded rollup proofs.
func (r *Runner) runAggregation(ctx context.Context, runReq RunRequest) RunResult {
	req, err := DecodeAggregationRequest(runReq.Body)
	if err != nil {
		return failedRunResult(runReq.ID, FailureCodeInvalidInput, err)
	}

	startBlock := req.RollupProofs[0].StartBlockNumber
	endBlock := req.RollupProofs[len(req.RollupProofs)-1].PublicInputs.EndBlockNumber

	result := r.prover.Prove(ctx, backend.Job{
		ID:         runReq.ID,
		Type:       runReq.Type,
		StartBlock: startBlock,
		EndBlock:   endBlock,
		Payload:    nil, // recursion guest input format undecided; mock passes no payload
	})
	if result.Status != backend.ResultStatusOK {
		return failedRunResult(runReq.ID, FailureCodeInternalError, proverErr(result))
	}
	return RunResult{
		ResponseBody: newAggregationResponse(result, startBlock, r.responseVersion()),
		Status:       RunStatusSuccess,
	}
}

// proverErr normalizes a non-OK prover result into an error.
func proverErr(result backend.Result) error {
	if result.Err != nil {
		return result.Err
	}
	return fmt.Errorf("prover returned status %s", result.Status)
}

func failedRunResult(id string, code FailureCode, err error) RunResult {
	return RunResult{
		ResponseBody: failureResponse(id, code, err),
		Status:       RunStatusFailed,
		FailureCode:  code,
		Err:          err,
	}
}
