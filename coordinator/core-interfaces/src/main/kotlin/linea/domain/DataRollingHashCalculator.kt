package linea.domain

/**
 * Folds a chunk hash into the data rolling hash: keccak256(parentDataRollingHash || chunkHash).
 */
fun interface DataRollingHashCalculator {
  fun fold(parentDataRollingHash: ByteArray, chunkHash: ByteArray): ByteArray
}
