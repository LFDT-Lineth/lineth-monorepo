package net.consensys.linea

import com.github.michaelbull.result.Result
import io.vertx.core.json.JsonObject
import linea.domain.BlockNumberAndHash
import tech.pegasys.teku.infrastructure.async.SafeFuture

class BlockTraces(
  val blockNumber: ULong,
  // val blockHash: Bytes32,
  val traces: JsonObject
)

data class TracesConflation(
  val startBlockNumber: ULong,
  val endBlockNumber: ULong,
  val traces: VersionedResult<JsonObject>
)

data class TracesFileIndex(val blockIndex: BlockNumberAndHash, val version: String) {
  val number get() = blockIndex.number
  val hash get() = blockIndex.hash
}

interface TracesRepositoryV1 {
  fun getTracesAsString(block: TracesFileIndex): SafeFuture<Result<String, TracesError>>
  fun getTraces(blocks: List<TracesFileIndex>): SafeFuture<Result<List<BlockTraces>, TracesError>>
}

interface ConflatedTracesRepository {
  fun findConflatedTraces(startBlockNumber: ULong, endBlockNumber: ULong, tracesVersion: String): SafeFuture<String?>
  fun saveConflatedTraces(conflation: TracesConflation): SafeFuture<String>
}
