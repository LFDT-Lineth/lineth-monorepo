package linea.domain

import kotlin.random.Random

/**
 * Used for testing purposes while real implementation is not ready
 * Shall be removed very soon
 */
class FakeBlobShnarfCalculator : BlobShnarfCalculator {
  override fun calculateShnarf(
    compressedData: ByteArray,
    parentStateRootHash: ByteArray,
    finalStateRootHash: ByteArray,
    prevShnarf: ByteArray,
    conflationOrder: BlockIntervals,
  ): ShnarfResult {
    return ShnarfResult(
      dataHash = Random.nextBytes(32),
      snarkHash = Random.nextBytes(32),
      expectedX = Random.nextBytes(32),
      expectedY = Random.nextBytes(32),
      expectedShnarf = Random.nextBytes(32),
      commitment = Random.nextBytes(48),
      kzgProofContract = Random.nextBytes(48),
      kzgProofSideCar = Random.nextBytes(48),
    )
  }
}
