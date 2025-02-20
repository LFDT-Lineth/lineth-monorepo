package linea.domain

data class FeeHistory(
  val oldestBlock: ULong,
  val baseFeePerGas: List<ULong>,
  val reward: List<List<ULong>>,
  val gasUsedRatio: List<Double>,
  val baseFeePerBlobGas: List<ULong>,
  val blobGasUsedRatio: List<Double>
) {
  fun blocksRange(): ClosedRange<ULong> {
    return oldestBlock..oldestBlock + ((reward.size - 1).toUInt())
  }
}
