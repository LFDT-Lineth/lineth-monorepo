package net.consensys.linea.web3j

import linea.web3j.Web3jBlobExtended
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.protocol.http.HttpService

class EthFeeHistoryBlobExtendedIntTest {
  @Test
  fun `eth_feeHistory response is deserialised correctly with blob data`() {
    val web3jBlobExtended = Web3jBlobExtended(HttpService("http://localhost:8445"))
    web3jBlobExtended.ethFeeHistoryWithBlob(
      blockCount = 5,
      newestBlock = DefaultBlockParameter.valueOf("latest"),
      rewardPercentiles = listOf(15.0)
    ).sendAsync()
      .thenApply { response ->
        assertThat(response).isNotNull
        val feeHistory = response.feeHistory
        assertThat(feeHistory.baseFeePerBlobGas).isNotNull
        assertThat(feeHistory.blobGasUsedRatio).isNotNull
        assertThat(feeHistory.gasUsedRatio).isNotNull
        assertThat(feeHistory.baseFeePerGas).isNotNull
        assertThat(feeHistory.reward).isNotNull
        assertThat(feeHistory.oldestBlock).isNotNull
      }
  }
}
