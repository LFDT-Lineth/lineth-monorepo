package linea.ethapi

import linea.domain.BlockNumberAndHash
import linea.domain.ExecutionPayload
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface ExecutionPayloadClient {
  /** Returns the payload for the exact block identity, failing if its required data is unavailable. */
  fun getExecutionPayload(block: BlockNumberAndHash): SafeFuture<ExecutionPayload>
}
