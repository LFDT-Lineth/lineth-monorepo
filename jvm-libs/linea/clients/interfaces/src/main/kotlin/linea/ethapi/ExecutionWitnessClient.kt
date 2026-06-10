package linea.ethapi

import linea.domain.BlockParameter
import linea.kotlin.byteArrayListEquals
import linea.kotlin.byteArrayListHashCode
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface ExecutionWitnessClient {
  fun getExecutionWitness(
    block: BlockParameter,
  ): SafeFuture<ExecutionWitness>
}

data class ExecutionWitness(
  val state: List<ByteArray>,
  val keys: List<ByteArray>,
  val codes: List<ByteArray>,
  val headers: List<ByteArray>,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false
    other as ExecutionWitness
    return state.byteArrayListEquals(other.state) &&
      keys.byteArrayListEquals(other.keys) &&
      codes.byteArrayListEquals(other.codes) &&
      headers.byteArrayListEquals(other.headers)
  }

  override fun hashCode(): Int {
    var result = state.byteArrayListHashCode()
    result = 31 * result + keys.byteArrayListHashCode()
    result = 31 * result + codes.byteArrayListHashCode()
    result = 31 * result + headers.byteArrayListHashCode()
    return result
  }
}

enum class ExecutionWitnessError {
  NULL_RESULT,
  PARSE_ERROR,
}

class ExecutionWitnessClientException(
  val errorType: ExecutionWitnessError,
  override val message: String,
  cause: Throwable? = null,
) : RuntimeException(message, cause)
