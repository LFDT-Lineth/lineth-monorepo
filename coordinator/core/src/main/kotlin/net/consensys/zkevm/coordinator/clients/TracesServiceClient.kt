package net.consensys.zkevm.coordinator.clients

import com.github.michaelbull.result.Result
import net.consensys.linea.errors.ErrorResponse
import net.consensys.linea.traces.TracesCounters
import tech.pegasys.teku.infrastructure.async.SafeFuture

enum class TracesServiceErrorType {
  BLOCK_MISSING_IN_CHAIN,
  BLOCK_RANGE_TOO_LARGE,
  INVALID_TRACES_VERSION,
  UNKNOWN_ERROR,
}

data class GetTracesCountersResponse(val tracesCounters: TracesCounters, val tracesEngineVersion: String)

data class GenerateTracesResponse(val tracesFileName: String, val tracesEngineVersion: String)

interface TracesCountersClientV2 {
  fun getTracesCounters(
    blockNumber: ULong,
  ): SafeFuture<Result<GetTracesCountersResponse, ErrorResponse<TracesServiceErrorType>>>
}

interface TracesConflationClientV2 {
  fun generateConflatedTracesToFile(
    startBlockNumber: ULong,
    endBlockNumber: ULong,
  ): SafeFuture<Result<GenerateTracesResponse, ErrorResponse<TracesServiceErrorType>>>
}
