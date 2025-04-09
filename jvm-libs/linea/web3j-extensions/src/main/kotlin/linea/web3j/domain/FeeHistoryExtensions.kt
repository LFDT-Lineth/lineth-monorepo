package linea.web3j.domain

import linea.domain.FeeHistory
import linea.kotlin.toULong
import org.web3j.protocol.core.methods.response.EthFeeHistory

fun EthFeeHistory.FeeHistory.toLineaDomain(): FeeHistory {
  return FeeHistory(
    oldestBlock = oldestBlock.toULong(),
    baseFeePerGas = baseFeePerGas.map { it.toULong() },
    reward = reward.map { it.map { it.toULong() } },
    gasUsedRatio = gasUsedRatio.map { it },
    baseFeePerBlobGas = listOf(0uL),
    blobGasUsedRatio = listOf(0.0)
  )
}
fun EthFeeHistory.FeeHistory.blocksRange(): ClosedRange<ULong> {
  return oldestBlock.toULong()..oldestBlock.toULong() + reward.size.toUInt() - 1u
}
