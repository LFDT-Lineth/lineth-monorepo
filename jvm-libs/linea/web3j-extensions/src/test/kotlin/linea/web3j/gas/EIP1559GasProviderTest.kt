package linea.web3j.gas

import linea.domain.BlockParameter
import linea.domain.FeeHistory
import linea.ethapi.EthApiClient
import linea.ethapi.FakeEthApiClient
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CopyOnWriteArrayList

class EIP1559GasProviderTest {
  private val config = EIP1559GasProvider.Config(
    gasLimit = 1_000_000uL,
    maxFeePerGasCap = 1_000uL,
    feeHistoryBlockCount = 2u,
    feeHistoryRewardPercentile = 15.0,
  )

  // rewards average to 20; maxFeePerGas = 2 * lastBaseFee + tip = 2 * 100 + 20
  private val feeHistory = FeeHistory(
    oldestBlock = 1uL,
    baseFeePerGas = listOf(90uL, 100uL),
    reward = listOf(listOf(10uL), listOf(30uL)),
    gasUsedRatio = listOf(0.5, 0.5),
    baseFeePerBlobGas = emptyList(),
    blobGasUsedRatio = emptyList(),
  )
  private val expectedFees = EIP1559GasFees(maxPriorityFeePerGas = 20uL, maxFeePerGas = 220uL)

  /** Real in-memory chain from FakeEthApiClient; only eth_feeHistory, which it does not implement, is supplied. */
  private class FeeHistoryEthApiClient(
    val chain: FakeEthApiClient = FakeEthApiClient(),
  ) : EthApiClient by chain {
    val feeHistoryResponses = CopyOnWriteArrayList<SafeFuture<FeeHistory>>()

    override fun ethFeeHistory(
      blockCount: Int,
      newestBlock: BlockParameter,
      rewardPercentiles: List<Double>,
    ): SafeFuture<FeeHistory> = SafeFuture<FeeHistory>().also { feeHistoryResponses.add(it) }
  }

  @Test
  fun `async fees complete only once fee history arrives and match the sync getter`() {
    val ethApiClient = FeeHistoryEthApiClient()
    // at block 0 the constructor does not fetch fees
    val gasProvider = EIP1559GasProvider(ethApiClient, config)
    ethApiClient.chain.setLatestBlockTag(10uL)

    val fees = gasProvider.getEIP1559GasFeesAsync()

    assertThat(fees).isNotDone()
    ethApiClient.feeHistoryResponses.single().complete(feeHistory)
    assertThat(fees.get()).isEqualTo(expectedFees)
    // same block: the sync getter is served from the cache and does not fetch again
    assertThat(gasProvider.getEIP1559GasFees()).isEqualTo(expectedFees)
    assertThat(ethApiClient.feeHistoryResponses).hasSize(1)
  }

  @Test
  fun `async fees are cached per block and refetched when the chain advances`() {
    val ethApiClient = FeeHistoryEthApiClient()
    val gasProvider = EIP1559GasProvider(ethApiClient, config)
    ethApiClient.chain.setLatestBlockTag(10uL)

    val first = gasProvider.getEIP1559GasFeesAsync()
    ethApiClient.feeHistoryResponses.single().complete(feeHistory)
    assertThat(first.get()).isEqualTo(expectedFees)
    assertThat(gasProvider.getEIP1559GasFeesAsync().get()).isEqualTo(expectedFees)
    assertThat(ethApiClient.feeHistoryResponses).hasSize(1)

    ethApiClient.chain.setLatestBlockTag(11uL)
    val refreshed = gasProvider.getEIP1559GasFeesAsync()
    assertThat(ethApiClient.feeHistoryResponses).hasSize(2)
    ethApiClient.feeHistoryResponses[1].complete(
      feeHistory.copy(baseFeePerGas = listOf(100uL, 200uL)),
    )
    assertThat(refreshed.get()).isEqualTo(EIP1559GasFees(maxPriorityFeePerGas = 20uL, maxFeePerGas = 420uL))
  }

  @Test
  fun `async fees are capped at maxFeePerGasCap`() {
    val ethApiClient = FeeHistoryEthApiClient()
    val gasProvider = EIP1559GasProvider(ethApiClient, config)
    ethApiClient.chain.setLatestBlockTag(10uL)

    val fees = gasProvider.getEIP1559GasFeesAsync()
    ethApiClient.feeHistoryResponses.single().complete(feeHistory.copy(baseFeePerGas = listOf(900uL, 900uL)))

    assertThat(fees.get()).isEqualTo(EIP1559GasFees(maxPriorityFeePerGas = 20uL, maxFeePerGas = config.maxFeePerGasCap))
  }
}
