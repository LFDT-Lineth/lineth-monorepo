package lineth.coordinator.clients

import com.github.michaelbull.result.Result
import lineth.clients.GenerateTracesResponse
import lineth.clients.TracesConflationVirtualBlockClientV1
import lineth.clients.TracesServiceErrorType
import lineth.error.ErrorResponse
import tech.pegasys.teku.infrastructure.async.SafeFuture

class FakeTracesConflationVirtualBlockClientV1 : TracesConflationVirtualBlockClientV1 {
  override fun generateVirtualBlockConflatedTracesToFile(
    blockNumber: ULong,
    transaction: ByteArray,
  ): SafeFuture<Result<GenerateTracesResponse, ErrorResponse<TracesServiceErrorType>>> {
    TODO("Not yet implemented")
  }
}
