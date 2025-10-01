package net.consensys.linea.ethereum.gaspricing.staticcap

import linea.OneKWei
import linea.domain.Block
import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.domain.createBlock
import linea.kotlin.decodeHex
import linea.web3j.ExtendedWeb3J
import net.consensys.linea.ethereum.gaspricing.MinerExtraDataV1
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
  val targetBlockNumber = 100UL
  val fakeBlock = createBlock(
    number = targetBlockNumber,
    extraData = MinerExtraDataV1(
      fixedCostInKWei = 1000U,
      variableCostInKWei = 10000U,
      ethGasPriceInKWei = 12000U,
    ).encode().decodeHex(),
  )
  val mockWeb3jClient = mock<ExtendedWeb3J> {
    on { ethGetBlock(eq(targetBlockNumber.toBlockParameter())) } doReturn
      SafeFuture.completedFuture(fakeBlock)
  }

  @Test
  fun test_getVariableCost() {
    val historicVariableCostProvider = HistoricVariableCostProviderImpl(
      web3jClient = mockWeb3jClient,
    )

    val latestVariableCost = historicVariableCostProvider.getVariableCost(targetBlockNumber).get()

    val expectedVariableCost = 10000.0 * OneKWei
    assertThat(latestVariableCost).isEqualTo(expectedVariableCost)
  }

  @Test
  fun test_getVariableCost_return_cached_variable_cost() {
    val historicVariableCostProvider = HistoricVariableCostProviderImpl(
      web3jClient = mockWeb3jClient,
    )

    // initialize the cache
    val expectedVariableCost = 10000.0 * OneKWei
    assertThat(
      historicVariableCostProvider.getVariableCost(targetBlockNumber).get(),
    ).isEqualTo(expectedVariableCost)

    // subsequent calls with the same block #100 should return the same value by the cache
    repeat(5) {
      assertThat(
        historicVariableCostProvider.getVariableCost(targetBlockNumber).get(),
      ).isEqualTo(expectedVariableCost)
    }

    // verified it only called ethGetBlock once
    verify(mockWeb3jClient, times(1))
      .ethGetBlock(eq(targetBlockNumber.toBlockParameter()))
  }

  @Test
  fun test_getVariableCost_throws_error_when_ethGetBlock_returns_null() {
    val mockWeb3jClient = mock<ExtendedWeb3J> {
      on { ethGetBlock(eq(targetBlockNumber.toBlockParameter())) } doReturn
        SafeFuture.completedFuture(null)
    }
    val historicVariableCostProvider = HistoricVariableCostProviderImpl(
      web3jClient = mockWeb3jClient,
    )

    assertThatThrownBy {
      historicVariableCostProvider.getVariableCost(targetBlockNumber).get()
    }.hasCause(IllegalStateException("Block $targetBlockNumber not found"))
  }

  @Test
  fun test_getVariableCost_throws_error_when_ethGetBlock_throws_error() {
    val expectedException = RuntimeException("Error from ethGetBlock")
    val mockWeb3jClient = mock<ExtendedWeb3J> {
      on { ethGetBlock(eq(targetBlockNumber.toBlockParameter())) } doReturn
        SafeFuture.failedFuture(expectedException)
    }
    val historicVariableCostProvider = HistoricVariableCostProviderImpl(
      web3jClient = mockWeb3jClient,
    )

    assertThatThrownBy {
      historicVariableCostProvider.getVariableCost(targetBlockNumber).get()
    }.hasCause(expectedException)
  }

  @Test
  fun test_getVariableCost_returns_zero_when_MinerExtraData_decode_throws_error() {
    val mockEthBlock = mock<Block> {
      // extra data hex string with unsupported version 0xFF
      on { extraData } doReturn "0xff000003e80000271000002ee000000000000000000000000000000000000000".decodeHex()
    }
    val mockWeb3jClient = mock<ExtendedWeb3J> {
      on { ethGetBlock(eq(targetBlockNumber.toBlockParameter())) } doReturn
        SafeFuture.completedFuture(mockEthBlock)
    }
    val historicVariableCostProvider = HistoricVariableCostProviderImpl(
      web3jClient = mockWeb3jClient,
    )

    assertThat(
      historicVariableCostProvider.getVariableCost(targetBlockNumber).get(),
    ).isEqualTo(0.0)
  }
}
