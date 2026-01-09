package net.consensys.linea.ethereum.gaspricing.staticcap

import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.domain.BlockWithTxHashes
import linea.ethapi.EthApiBlockClient
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import org.mockito.kotlin.any
import org.mockito.kotlin.argThat
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.times
import org.mockito.kotlin.verify
import tech.pegasys.teku.infrastructure.async.SafeFuture

class L2CalldataSizeAccumulatorImplTest {
  private val config = L2CalldataSizeAccumulatorImpl.Config(
    blockSizeNonCalldataOverhead = 540u,
    calldataSizeBlockCount = 5u,
  )
  private val targetBlockNumber = 100UL

  @Test
  fun test_getSumOfL2CalldataSize() {
    val mockBlock = mock<BlockWithTxHashes> {
      on { size } doReturn 10540uL
    }
    val mockEthApiBlockClient = mock<EthApiBlockClient> {
      on { ethGetBlockByNumberTxHashes(any()) } doReturn SafeFuture.completedFuture(mockBlock)
    }
    val l2CalldataSizeAccumulator = L2CalldataSizeAccumulatorImpl(
      config = config,
      ethApiBlockClient = mockEthApiBlockClient,
    )

    val sumOfL2CalldataSize = l2CalldataSizeAccumulator.getSumOfL2CalldataSize(targetBlockNumber).get()

    (0..4).forEach {
      verify(mockEthApiBlockClient, times(1))
        .ethGetBlockByNumberTxHashes(eq((targetBlockNumber.toLong() - it).toBlockParameter()))
    }

    val expectedCalldataSize = ((10540 - 540) * 5).toULong()
    assertThat(sumOfL2CalldataSize).isEqualTo(expectedCalldataSize)
  }

  @Test
  fun test_getSumOfL2CalldataSize_return_cached_sum_of_l2_calldata() {
    val mockBlock = mock<BlockWithTxHashes> {
      on { size } doReturn 10540uL
    }

    val mockEthApiBlockClient = mock<EthApiBlockClient> {
      on {
        ethGetBlockByNumberTxHashes(argThat { parameter -> parameter.getNumber() in 96uL..100uL })
      } doReturn SafeFuture.completedFuture(mockBlock)
    }
    val l2CalldataSizeAccumulator = L2CalldataSizeAccumulatorImpl(
      config = config,
      ethApiBlockClient = mockEthApiBlockClient,
    )

    // initialize the cache
    val sumOfL2CalldataSize = l2CalldataSizeAccumulator.getSumOfL2CalldataSize(targetBlockNumber).get()
    val expectedCalldataSize = ((10540 - 540) * 5).toULong()
    assertThat(sumOfL2CalldataSize).isEqualTo(expectedCalldataSize)

    // subsequent calls with the same block #100 should return the same value from the cache
    repeat(5) {
      assertThat(
        l2CalldataSizeAccumulator.getSumOfL2CalldataSize(targetBlockNumber).get(),
      ).isEqualTo(expectedCalldataSize)
    }

    // subsequent calls should not invoke ethGetBlockSizeByNumber
    (96..100).forEach { it ->
      verify(mockEthApiBlockClient, times(1))
        .ethGetBlockByNumberTxHashes(eq(it.toBlockParameter()))
    }
  }

  @Test
  fun test_getSumOfL2CalldataSize_when_each_calldata_size_is_zero() {
    val mockBlock = mock<BlockWithTxHashes> {
      on { size } doReturn 0uL
    }
    val mockEthApiBlockClient = mock<EthApiBlockClient> {
      on { ethGetBlockByNumberTxHashes(any()) } doReturn SafeFuture.completedFuture(mockBlock)
    }
    val l2CalldataSizeAccumulator = L2CalldataSizeAccumulatorImpl(
      config = config,
      ethApiBlockClient = mockEthApiBlockClient,
    )

    val sumOfL2CalldataSize = l2CalldataSizeAccumulator.getSumOfL2CalldataSize(targetBlockNumber).get()

    (0..4).forEach {
      verify(mockEthApiBlockClient, times(1))
        .ethGetBlockByNumberTxHashes(eq((targetBlockNumber.toLong() - it).toBlockParameter()))
    }

    assertThat(sumOfL2CalldataSize).isEqualTo(0uL)
  }

  @Test
  fun test_getSumOfL2CalldataSize_throws_exception_when_ethGetBlockSizeByNumber_throws_exception() {
    val expectedException = RuntimeException("Failed for testing")
    val mockEthApiBlockClient = mock<EthApiBlockClient> {
      on { ethGetBlockByNumberTxHashes(any()) } doReturn SafeFuture.failedFuture(expectedException)
    }
    val l2CalldataSizeAccumulator = L2CalldataSizeAccumulatorImpl(
      config = config,
      ethApiBlockClient = mockEthApiBlockClient,
    )

    assertThatThrownBy {
      l2CalldataSizeAccumulator.getSumOfL2CalldataSize(targetBlockNumber).get()
    }.hasCause(expectedException)
  }

  @Test
  fun test_getSumOfL2CalldataSize_returns_zero_when_target_block_number_is_less_than_calldataSizeBlockCount() {
    val mockBlock = mock<BlockWithTxHashes> {
      on { size } doReturn 10540uL
    }
    val mockEthApiBlockClient = mock<EthApiBlockClient> {
      on { ethGetBlockByNumberTxHashes(any()) } doReturn SafeFuture.completedFuture(mockBlock)
    }
    val l2CalldataSizeAccumulator = L2CalldataSizeAccumulatorImpl(
      config = config,
      ethApiBlockClient = mockEthApiBlockClient,
    )

    val sumOfL2CalldataSize = l2CalldataSizeAccumulator.getSumOfL2CalldataSize(1UL).get()

    verify(mockEthApiBlockClient, times(0)).ethGetBlockByNumberTxHashes(any())

    assertThat(sumOfL2CalldataSize).isEqualTo(0uL)
  }

  @Test
  fun test_getSumOfL2CalldataSize_returns_zero_when_calldataSizeBlockCount_is_zero() {
    val mockBlock = mock<BlockWithTxHashes> {
      on { size } doReturn 10540uL
    }
    val mockEthApiBlockClient = mock<EthApiBlockClient> {
      on { ethGetBlockByNumberTxHashes(any()) } doReturn SafeFuture.completedFuture(mockBlock)
    }
    val l2CalldataSizeAccumulator = L2CalldataSizeAccumulatorImpl(
      config = L2CalldataSizeAccumulatorImpl.Config(
        blockSizeNonCalldataOverhead = 540u,
        calldataSizeBlockCount = 0u,
      ),
      ethApiBlockClient = mockEthApiBlockClient,
    )

    val sumOfL2CalldataSize = l2CalldataSizeAccumulator.getSumOfL2CalldataSize(targetBlockNumber).get()

    verify(mockEthApiBlockClient, times(0)).ethGetBlockByNumberTxHashes(any())

    assertThat(sumOfL2CalldataSize).isEqualTo(0uL)
  }
}
