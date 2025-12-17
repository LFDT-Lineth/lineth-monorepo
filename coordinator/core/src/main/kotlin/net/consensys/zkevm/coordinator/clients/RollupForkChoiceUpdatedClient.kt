package net.consensys.zkevm.coordinator.clients

import com.github.michaelbull.result.Result
import linea.domain.BlockNumberAndHash
import net.consensys.linea.errors.ErrorResponse
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
