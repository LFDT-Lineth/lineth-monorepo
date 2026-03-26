package dataavailability

import "github.com/consensys/linea-monorepo/prover/backend/blobsubmission"

// The decompression proof request is conveniently exactly the same as the
// response of the blobsubmission. Some fields are not used, but it simplifies
// the code.
type Request = blobsubmission.Response
