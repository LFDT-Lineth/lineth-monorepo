package net.consensys.zkevm.coordinator.clients

import com.github.michaelbull.result.Result
import linea.clients.GenerateTracesResponse
import linea.clients.TracesConflationVirtualBlockClientV1
import linea.clients.TracesServiceErrorType
import linea.error.ErrorResponse
import tech.pegasys.teku.infrastructure.async.SafeFuture

class FakeTracesConflationVirtualBlockClientV1 : TracesConflationVirtualBlockClientV1 {
  override fun generateVirtualBlockConflatedTracesToFile(
    blockNumber: ULong,
    transaction: ByteArray,
  ): SafeFuture<Result<GenerateTracesResponse, ErrorResponse<TracesServiceErrorType>>> {
    TODO("Not yet implemented")
  }
}
