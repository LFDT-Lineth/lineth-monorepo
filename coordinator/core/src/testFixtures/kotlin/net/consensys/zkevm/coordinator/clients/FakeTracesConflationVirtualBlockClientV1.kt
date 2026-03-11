package net.consensys.zkevm.coordinator.clients

import com.github.michaelbull.result.Result
import net.consensys.linea.errors.ErrorResponse
import tech.pegasys.teku.infrastructure.async.SafeFuture

class FakeTracesConflationVirtualBlockClientV1 : TracesConflationVirtualBlockClientV1 {
  override fun generateVirtualBlockConflatedTracesToFile(
    blockNumber: ULong,
    transaction: ByteArray,
  ): SafeFuture<Result<GenerateTracesResponse, ErrorResponse<TracesServiceErrorType>>> {
    TODO("Not yet implemented")
  }
}
