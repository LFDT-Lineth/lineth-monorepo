package net.consensys.linea

import com.github.michaelbull.result.Result
import io.vertx.core.json.JsonObject
import linea.domain.BlockNumberAndHash
import net.consensys.linea.traces.TracesCounters
import tech.pegasys.teku.infrastructure.async.SafeFuture

data class VersionedResult<T>(val version: String, val result: T)

data class BlockCounters(val tracesCounters: TracesCounters, val blockL1Size: UInt)

interface TracesCountingServiceV1 {
  fun getBlockTracesCounters(
    block: BlockNumberAndHash,
    version: String
  ): SafeFuture<Result<VersionedResult<BlockCounters>, TracesError>>
}
interface TracesConflationServiceV1 {
  fun getConflatedTraces(
    blocks: List<BlockNumberAndHash>,
    version: String
  ): SafeFuture<Result<VersionedResult<JsonObject>, TracesError>>

  fun generateConflatedTracesToFile(
    blocks: List<BlockNumberAndHash>,
    version: String
  ): SafeFuture<Result<VersionedResult<String>, TracesError>>
}
