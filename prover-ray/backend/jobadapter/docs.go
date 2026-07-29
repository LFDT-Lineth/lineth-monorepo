// Package jobadapter adapts coordinator proof requests into backend Jobs.
//
// The code is split into three roles:
//
//   - The filesystem subpackage owns the file queue: finding request files,
//     claiming work, writing responses, and archiving completed requests.
//   - Runner owns the request-to-proof flow: decoding the coordinator request,
//     SSZ-encoding payloads, calling the prover, and shaping the response.
//   - Prover is the proving engine behind the adapter: it receives a
//     backend.Job and returns the proof result.
//
// This keeps file handling separate from request decoding and proving.
package jobadapter
