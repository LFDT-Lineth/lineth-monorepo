package linea.ethapi

import linea.domain.Block
import linea.domain.ExecutionPayload
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface ExecutionPayloadClient {
  /** Enriches an acquired block with its verified BAL */
  fun getExecutionPayload(block: Block): SafeFuture<ExecutionPayload>
}
