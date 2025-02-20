package net.consensys.linea.web3j

import linea.domain.FeeHistory
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.web3j.protocol.core.methods.response.EthFeeHistory

class FeeHistoryExtensionsTest {
  @Test
  fun mapSingleEntry() {
    val feeHistory = EthFeeHistory.FeeHistory().apply {
      this.setOldestBlock("0xf")
      this.setBaseFeePerGas(listOf("0xa1", "0xa2"))
      this.setReward(listOf(listOf("0xba1", "0xba2")))
      this.gasUsedRatio = listOf(0.25)
    }

    assertThat(feeHistory.toLineaDomain()).isEqualTo(
      FeeHistory(
        oldestBlock = "f".toULong(16),
        baseFeePerGas = listOf("a1", "a2").map { it.toULong(16) },
        reward = listOf(listOf("ba1", "ba2").map { it.toULong(16) }),
        gasUsedRatio = listOf(0.25),
        baseFeePerBlobGas = listOf(0uL),
        blobGasUsedRatio = listOf(0.0)
      )
    )
  }

  @Test
  fun mapMultipleEntries() {
    val feeHistory = EthFeeHistory.FeeHistory().apply {
      this.setOldestBlock("0xf")
      this.setBaseFeePerGas(listOf("0xa1", "0xa2", "0xa3"))
      this.setReward(
        listOf(
          listOf("0xba1", "0xba2"),
          listOf("0xbb1", "0xbb2")
        )
      )
      this.gasUsedRatio = listOf(0.25, 0.75)
    }

    assertThat(feeHistory.toLineaDomain()).isEqualTo(
      FeeHistory(
        oldestBlock = "f".toULong(16),
        baseFeePerGas = listOf("a1", "a2", "a3").map { it.toULong(16) },
        reward = listOf(
          listOf("ba1", "ba2").map { it.toULong(16) },
          listOf("bb1", "bb2").map { it.toULong(16) }
        ),
        gasUsedRatio = listOf(0.25, 0.75),
        baseFeePerBlobGas = listOf(0uL),
        blobGasUsedRatio = listOf(0.0)
      )
    )
  }
}
