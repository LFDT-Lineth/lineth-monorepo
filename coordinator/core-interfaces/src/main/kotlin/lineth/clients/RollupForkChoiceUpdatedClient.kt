package lineth.clients

import com.github.michaelbull.result.Result
import lineth.domain.BlockNumberAndHash
import lineth.error.ErrorResponse
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
