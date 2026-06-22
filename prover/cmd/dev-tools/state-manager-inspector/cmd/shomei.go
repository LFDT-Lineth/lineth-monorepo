package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"text/template"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

const (
	// Content-type : application/json
	applicationJSONContentType = "application/json"
	// The template string of the state-manager transition proofs
	getZkEVMStateMerkleProofV0RequestTmplStr = `{
	"jsonrpc": "2.0",
	"method": "rollup_getZkEVMStateMerkleProofV0",
	"params": [
		{
			"startBlockNumber": "{{.StartBlockNumberHex}}",
			"endBlockNumber": "{{.EndBlockNumberHex}}"
		}
	],
	"id": 1
}`
)

var (
	// The template for the state-manager transition proof. generated from the
	// template string.
	getZkEVMStateMerkleProofV0RequestTmpl = template.Must(
		template.New("getZkEVMStateMerkleProofV0Request").
			Parse(getZkEVMStateMerkleProofV0RequestTmplStr),
	)
)

// shomeiClient implements all the functionalities to instantiate a shomei client
type shomeiClient struct {
	hostport string
	client   *http.Client
	// Everytime a request is fired, the client is going to wait for the next
	// tick before he can send the request. The intent is to ensure that we are
	// not DDOS-ing the remote Shomei server.
	throttler *rate.Limiter
	// Maximal number of retries when Shomei returns a 5XX failure code
	maxRetries int
}

// zkEVMStateMerkleProofV0 represents a Shomei request to collect the Merkle
// proofs justifying the transition of a given range of blocks.
type zkEVMStateMerkleProofV0Req struct {
	StartBlockNumber    int
	EndBlockNumber      int
	StartBlockNumberHex string
	EndBlockNumberHex   string
}

// Sends a request to collect the zk state proofs for a conflated sequence of
// blocks.
func (sc *shomeiClient) fetchStateTransitionProofs(
	ctx context.Context,
	req *zkEVMStateMerkleProofV0Req,
) ([]byte, error) {

	// common error msg strings
	var (
		funcCtx = fmt.Sprintf("inFetchStateTransition for %++v", req)
	)

	req.EndBlockNumberHex = fmt.Sprintf("0x%x", req.EndBlockNumber)
	req.StartBlockNumberHex = fmt.Sprintf("0x%x", req.StartBlockNumber)

	// Execute the request template to generate the body of the request. If
	// this fails it means that the request is invalid.
	body := &bytes.Buffer{}
	if err := getZkEVMStateMerkleProofV0RequestTmpl.Execute(body, req); err != nil {
		return nil, fmt.Errorf("%s : while instantiating the template : %s", funcCtx, err)
	}

	// The request is performed in a retry loop for shomei
	for _tryCount := 0; _tryCount < sc.maxRetries; _tryCount++ {

		// It will return an error if it does not get a token from the bucket in
		// in less than one second. Not a hard error, we can retry.
		if err := sc.throttler.Wait(ctx); err != nil {
			logrus.Tracef("%s : while acquiring the throttler : %s", funcCtx, err)
			continue
		}

		// @alex: there is no need to close the body here as the reader is a
		// byte buffer and not a file or anything that needs to be closed.
		resp, err := sc.client.Post(
			sc.hostport,
			applicationJSONContentType,
			body,
		)

		if err != nil {
			// An error occurring here can be different things. Network error,
			// bad host:port. When that happens, we log an error and retry. It
			// is not a very fine grained approach as it will likely retry hard
			// errors most of the time. But at least, it will catch all network
			// errors.
			logrus.Errorf("%s : while sending the request : %s", funcCtx, err)
			continue
		}

		// Important to avoid leaks. The result has to be rebuffered before
		// returning the reader.
		defer resp.Body.Close()

		// The happy path
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			b, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("%s : could not read response body %w", funcCtx, err)
			}
			return b, nil
		}

		// This consumes the request body, so we can only Read it once we are
		// sure it will not be returned. That's why this must be done only after
		// the happy path has been ruled out. We do not process the error
		// because we only read the body to construct the appropriate error
		// message
		respMsgBytes, _ := io.ReadAll(resp.Body)

		// We may only retry on a 500 code
		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			logrus.Errorf(
				"%s, got an invalid response : %s, (status-code: %s), retrying",
				funcCtx, string(respMsgBytes), resp.Status,
			)
			continue
		}

		// Else, this is an unexpected error code. We return an error. Note that
		// since we are crafting the requests ourselves a 4XX is unexpected.
		return nil, fmt.Errorf(
			"%s : unexpected status code %d, response body = %s",
			funcCtx, resp.StatusCode, string(respMsgBytes),
		)
	}

	return nil, fmt.Errorf("%s : exceeded the number of retries", funcCtx)
}
