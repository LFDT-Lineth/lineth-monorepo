package lineth.ethereum.gaspricing.staticcap

import lineth.OneKWei
import lineth.domain.BlockWithTxHashes
import lineth.domain.createBlock
import lineth.domain.toBlockParameter
import lineth.domain.toBlockWithRandomTxHashes
import lineth.ethapi.EthApiBlockClient
import lineth.ethereum.gaspricing.MinerExtraDataV1
import lineth.kotlin.decodeHex
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.times
import org.mockito.kotlin.verify
import tech.pegasys.teku.infrastructure.async.SafeFuture

class HistoricVariableCostProviderImplTest {
  val targetBlockNumber = 100UL.toBlockParameter()
  val fakeBlock = createBlock(
    number = targetBlockNumber.number,
    extraData = MinerExtraDataV1(
      fixedCostInKWei = 1000U,
      variableCostInKWei = 10000U,
      ethGasPriceInKWei = 12000U,
    ).encode().decodeHex(),
  ).toBlockWithRandomTxHashes()
  val mockEthApiBlockClient = mock<EthApiBlockClient> {
    on { ethFindBlockByNumberTxHashes(eq(targetBlockNumber)) } doReturn SafeFuture.completedFuture(fakeBlock)
  }

  @Test
  fun test_getVariableCost() {
    val historicVariableCostProvider = HistoricVariableCostProviderImpl(
      ethApiBlockClient = mockEthApiBlockClient,
    )

    val latestVariableCost = historicVariableCostProvider.getVariableCost(targetBlockNumber.number).get()

    val expectedVariableCost = 10000.0 * OneKWei
    assertThat(latestVariableCost).isEqualTo(expectedVariableCost)
  }

  @Test
  fun test_getVariableCost_return_cached_variable_cost() {
    val historicVariableCostProvider = HistoricVariableCostProviderImpl(
      ethApiBlockClient = mockEthApiBlockClient,
    )

    // initialize the cache
    val expectedVariableCost = 10000.0 * OneKWei
    assertThat(
      historicVariableCostProvider.getVariableCost(targetBlockNumber.number).get(),
    ).isEqualTo(expectedVariableCost)

    // subsequent calls with the same block #100 should return the same value by the cache
    repeat(5) {
      assertThat(
        historicVariableCostProvider.getVariableCost(targetBlockNumber.number).get(),
      ).isEqualTo(expectedVariableCost)
    }

    // verified it only called ethGetBlock once
    verify(mockEthApiBlockClient, times(1))
      .ethFindBlockByNumberTxHashes(eq(targetBlockNumber))
  }

  @Test
  fun test_getVariableCost_throws_error_when_ethGetBlock_returns_null() {
    val mockEthApiBlockClient = mock<EthApiBlockClient> {
      on { ethFindBlockByNumberTxHashes(eq(targetBlockNumber)) } doReturn
        SafeFuture.completedFuture(null)
    }
    val historicVariableCostProvider = HistoricVariableCostProviderImpl(
      ethApiBlockClient = mockEthApiBlockClient,
    )

    assertThatThrownBy {
      historicVariableCostProvider.getVariableCost(targetBlockNumber.number).get()
    }.hasCause(IllegalStateException("Block $targetBlockNumber not found"))
  }

  @Test
  fun test_getVariableCost_throws_error_when_ethGetBlock_throws_error() {
    val expectedException = RuntimeException("Error from ethGetBlock")
    val mockEthApiBlockClient = mock<EthApiBlockClient> {
      on { ethFindBlockByNumberTxHashes(eq(targetBlockNumber)) } doReturn
        SafeFuture.failedFuture(expectedException)
    }
    val historicVariableCostProvider = HistoricVariableCostProviderImpl(
      ethApiBlockClient = mockEthApiBlockClient,
    )

    assertThatThrownBy {
      historicVariableCostProvider.getVariableCost(targetBlockNumber.number).get()
    }.hasCause(expectedException)
  }

  @Test
  fun test_getVariableCost_returns_zero_when_MinerExtraData_decode_throws_error() {
    val mockEthBlock = mock<BlockWithTxHashes> {
      // extra data hex string with unsupported version 0xFF
      on { extraData } doReturn "0xff000003e80000271000002ee000000000000000000000000000000000000000".decodeHex()
    }
    val mockEthApiBlockClient = mock<EthApiBlockClient> {
      on { ethFindBlockByNumberTxHashes(eq(targetBlockNumber)) } doReturn
        SafeFuture.completedFuture(mockEthBlock)
    }
    val historicVariableCostProvider = HistoricVariableCostProviderImpl(
      ethApiBlockClient = mockEthApiBlockClient,
    )

    assertThat(
      historicVariableCostProvider.getVariableCost(targetBlockNumber.number).get(),
    ).isEqualTo(0.0)
  }
}
