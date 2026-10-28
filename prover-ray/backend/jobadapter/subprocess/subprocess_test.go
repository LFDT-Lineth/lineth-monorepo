package subprocess_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/jobadapter"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/jobadapter/subprocess"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fake child modes.
const (
	modeSuccess        = "success"
	modeHandledFailure = "handled-failure"
	modeNotJSON        = "not-json"
	modeCrash          = "crash"
	modeOOM            = "oom"
	modeHang           = "hang"
)

// fakeChildEnv, when set, makes the test binary run as a fake child in that mode.
const fakeChildEnv = "SUBPROCESS_FAKE_CHILD"

func TestMain(m *testing.M) {
	if mode := os.Getenv(fakeChildEnv); mode != "" {
		os.Exit(fakeChild(mode))
	}
	os.Exit(m.Run())
}

// fakeChild echoes its arguments and stdin so tests can check what the parent
// passed.
func fakeChild(mode string) int {
	body, _ := io.ReadAll(os.Stdin)
	args := strings.Join(os.Args[1:], " ")
	switch mode {
	case modeSuccess:
		fmt.Printf(`{"args":%q,"body":%q}`, args, string(body))
		return subprocess.ExitSuccess
	case modeHandledFailure:
		fmt.Print(`{"jobId":"j","status":"failed","failureCode":"invalid_input","error":"bad payload"}`)
		return subprocess.ExitFailure
	case modeNotJSON:
		fmt.Print("not json")
		return subprocess.ExitSuccess
	case modeCrash:
		return 2
	case modeOOM:
		_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
		time.Sleep(time.Minute)
		return 0
	case modeHang:
		time.Sleep(time.Minute)
		return 0
	}
	return 99
}

func fakeCommand(t *testing.T, mode string) subprocess.Command {
	t.Helper()
	t.Setenv(fakeChildEnv, mode)
	bin, err := os.Executable()
	require.NoError(t, err)
	return subprocess.Command{Bin: bin, ConfigPath: "/cfg.toml"}
}

func sampleRequest() jobadapter.RunRequest {
	return jobadapter.RunRequest{ID: "59144/execution/10/11", Type: backend.ProofTypeL2Execution, Body: []byte(`{"x":1}`)}
}

func TestSpawn_PassesRequestOnStdinAndFlags(t *testing.T) {
	out, err := fakeCommand(t, modeSuccess).Spawn(context.Background(), sampleRequest())
	require.NoError(t, err)
	assert.Equal(t, subprocess.ExitSuccess, out.ExitCode)

	var echoed struct{ Args, Body string }
	require.NoError(t, json.Unmarshal(out.Response, &echoed), "stdout must be the child's response only")
	assert.Equal(t, `{"x":1}`, echoed.Body, "the request bytes reach the child on stdin")
	assert.Equal(t,
		"prove --config /cfg.toml --type l2-execution --id 59144/execution/10/11 --in - --out -",
		echoed.Args)
}

func TestSpawn_ExitCodes(t *testing.T) {
	tests := map[string]int{
		modeHandledFailure: subprocess.ExitFailure,
		modeCrash:          2,
		modeOOM:            subprocess.ExitOOM,
	}
	for mode, want := range tests {
		t.Run(mode, func(t *testing.T) {
			out, err := fakeCommand(t, mode).Spawn(context.Background(), sampleRequest())
			require.NoError(t, err, "a child that ran and failed is not a spawn error")
			assert.Equal(t, want, out.ExitCode)
		})
	}
}

func TestSpawn_MissingBinary(t *testing.T) {
	_, err := subprocess.Command{Bin: "/does/not/exist", ConfigPath: "/cfg.toml"}.Spawn(context.Background(), sampleRequest())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "starting prover child")
}

func TestSpawn_CancelIsNotOOM(t *testing.T) {
	cmd := fakeCommand(t, modeHang)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	_, err := cmd.Spawn(ctx, sampleRequest())
	require.Error(t, err)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestRun_Success(t *testing.T) {
	res := fakeCommand(t, modeSuccess).Run(context.Background(), sampleRequest())
	require.Equal(t, jobadapter.RunStatusSuccess, res.Status, "err: %v", res.Err)
	raw, ok := res.ResponseBody.(json.RawMessage)
	require.True(t, ok, "the response is passed through as raw JSON")
	assert.True(t, json.Valid(raw))
	assert.Empty(t, res.FailureCode)
}

func TestRun_Failures(t *testing.T) {
	tests := map[string]struct {
		code    jobadapter.FailureCode
		errText string
		hasBody bool
	}{
		modeHandledFailure: {jobadapter.FailureCodeInvalidInput, "bad payload", true},
		modeCrash:          {jobadapter.FailureCodeInternalError, "exited with code 2", false},
		modeOOM:            {jobadapter.FailureCodeOOM, "exited with code 137", false},
		modeNotJSON:        {jobadapter.FailureCodeInternalError, "without a valid JSON response", false},
	}
	for mode, tc := range tests {
		t.Run(mode, func(t *testing.T) {
			res := fakeCommand(t, mode).Run(context.Background(), sampleRequest())
			assert.Equal(t, jobadapter.RunStatusFailed, res.Status)
			assert.Equal(t, tc.code, res.FailureCode)
			require.Error(t, res.Err)
			assert.Contains(t, res.Err.Error(), tc.errText)
			assert.Equal(t, tc.hasBody, res.ResponseBody != nil)
		})
	}
}

func TestRun_CancelIsInternalError(t *testing.T) {
	cmd := fakeCommand(t, modeHang)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	res := cmd.Run(ctx, sampleRequest())
	assert.Equal(t, jobadapter.RunStatusFailed, res.Status)
	assert.Equal(t, jobadapter.FailureCodeInternalError, res.FailureCode,
		"a cancelled proof is not an OOM; it must not escalate the job's tier")
}

func TestFailureFor(t *testing.T) {
	tests := map[string]struct {
		out  subprocess.Output
		want jobadapter.FailureCode
	}{
		"code from body wins over exit code": {
			subprocess.Output{Response: []byte(`{"failureCode":"invalid_input"}`), ExitCode: subprocess.ExitOOM},
			jobadapter.FailureCodeInvalidInput,
		},
		"oom kill":          {subprocess.Output{ExitCode: subprocess.ExitOOM}, jobadapter.FailureCodeOOM},
		"other crash":       {subprocess.Output{ExitCode: 2}, jobadapter.FailureCodeInternalError},
		"body without code": {subprocess.Output{Response: []byte(`{}`), ExitCode: 1}, jobadapter.FailureCodeInternalError},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, _ := subprocess.FailureFor(tc.out)
			assert.Equal(t, tc.want, got)
		})
	}
}
