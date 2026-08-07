package lineth.ethapi

import lineth.domain.BlockParameter
import lineth.kotlin.byteArrayListEquals
import lineth.kotlin.byteArrayListHashCode
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface ExecutionWitnessClient {
  /**
   * Returns the execution witness for [block], or `null` when the node has no witness for it
   * (e.g. an unknown block hash). JSON-RPC error responses are rejected with
   * [lineth.error.JsonRpcErrorResponseException].
   */
  fun getExecutionWitness(
    block: BlockParameter,
  ): SafeFuture<ExecutionWitness?>
}

data class ExecutionWitness(
  val state: List<ByteArray>,
  val codes: List<ByteArray>,
  val headers: List<ByteArray>,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false
    other as ExecutionWitness
    return state.byteArrayListEquals(other.state) &&
      codes.byteArrayListEquals(other.codes) &&
      headers.byteArrayListEquals(other.headers)
  }

  override fun hashCode(): Int {
    var result = state.byteArrayListHashCode()
    result = 31 * result + codes.byteArrayListHashCode()
    result = 31 * result + headers.byteArrayListHashCode()
    return result
  }
}
