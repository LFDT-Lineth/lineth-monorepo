package linea.executionwitness

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
    return byteArrayListsEqual(state, other.state) &&
      byteArrayListsEqual(keys, other.keys) &&
      byteArrayListsEqual(codes, other.codes) &&
      byteArrayListsEqual(headers, other.headers)
  }

  override fun hashCode(): Int {
    var result = byteArrayListHashCode(state)
    result = 31 * result + byteArrayListHashCode(keys)
    result = 31 * result + byteArrayListHashCode(codes)
    result = 31 * result + byteArrayListHashCode(headers)
    return result
  }

  private fun byteArrayListsEqual(a: List<ByteArray>, b: List<ByteArray>): Boolean {
    if (a.size != b.size) return false
    return a.indices.all { i -> a[i].contentEquals(b[i]) }
  }

  private fun byteArrayListHashCode(list: List<ByteArray>): Int {
    return list.fold(0) { acc, bytes -> 31 * acc + bytes.contentHashCode() }
  }
}

enum class ExecutionWitnessError {
  NULL_RESULT,
  RPC_ERROR,
  PARSE_ERROR,
}
