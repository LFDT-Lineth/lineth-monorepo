package linea.clients

import com.github.michaelbull.result.Result
import linea.domain.BlockNumberAndHash
import linea.error.ErrorResponse
import tech.pegasys.teku.infrastructure.async.SafeFuture

enum class RollupForkChoiceUpdatedError {
  UNKNOWN,
}

data class RollupForkChoiceUpdatedResponse(val result: String)

interface RollupForkChoiceUpdatedClient {
  fun rollupForkChoiceUpdated(
    finalizedBlockNumberAndHash: BlockNumberAndHash,
  ): SafeFuture<Result<RollupForkChoiceUpdatedResponse, ErrorResponse<RollupForkChoiceUpdatedError>>>
}
