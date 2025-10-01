package net.consensys.linea.ethereum.gaspricing.staticcap

import linea.web3j.ExtendedWeb3J
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import org.mockito.ArgumentMatchers
import org.mockito.kotlin.any
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.times
import org.mockito.kotlin.verify
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger

class L2CalldataSizeAccumulatorImplTest {
  private val config = L2CalldataSizeAccumulatorImpl.Config(
    blockSizeNonCalldataOverhead = 540u,
    calldataSizeBlockCount = 5u,
  )
  private val targetBlockNumber = 100UL

  @Test
  fun test_getSumOfL2CalldataSize() {
    val mockWeb3jClient = mock<ExtendedWeb3J> {
      on { ethGetBlockSizeByNumber(any()) } doReturn SafeFuture.completedFuture(10540.toBigInteger())
    }
    val l2CalldataSizeAccumulator = L2CalldataSizeAccumulatorImpl(
      config = config,
      web3jClient = mockWeb3jClient,
    )

    val sumOfL2CalldataSize = l2CalldataSizeAccumulator.getSumOfL2CalldataSize(targetBlockNumber).get()

    (0..4).forEach {
      verify(mockWeb3jClient, times(1))
        .ethGetBlockSizeByNumber(eq(targetBlockNumber.toLong() - it))
    }

    val expectedCalldataSize = (10540 - 540) * 5
    assertThat(sumOfL2CalldataSize).isEqualTo(expectedCalldataSize.toBigInteger())
  }

  @Test
  fun test_getSumOfL2CalldataSize_return_cached_sum_of_l2_calldata() {
    val mockWeb3jClient = mock<ExtendedWeb3J> {
      on {
        ethGetBlockSizeByNumber(
          ArgumentMatchers.longThat {
            (96..100).contains(it)
          },
        )
      } doReturn SafeFuture.completedFuture(10540.toBigInteger())
    }
    val l2CalldataSizeAccumulator = L2CalldataSizeAccumulatorImpl(
      config = config,
      web3jClient = mockWeb3jClient,
    )

    // initialize the cache
    val sumOfL2CalldataSize = l2CalldataSizeAccumulator.getSumOfL2CalldataSize(targetBlockNumber).get()
    val expectedCalldataSize = (10540 - 540) * 5
    assertThat(sumOfL2CalldataSize).isEqualTo(expectedCalldataSize.toBigInteger())

    // subsequent calls with the same block #100 should return the same value from the cache
    repeat(5) {
      assertThat(
        l2CalldataSizeAccumulator.getSumOfL2CalldataSize(targetBlockNumber).get(),
      ).isEqualTo(expectedCalldataSize)
    }

    // subsequent calls should not invoke ethGetBlockSizeByNumber
    (96..100).forEach { it ->
      verify(mockWeb3jClient, times(1))
        .ethGetBlockSizeByNumber(eq(it.toLong()))
    }
  }

  @Test
  fun test_getSumOfL2CalldataSize_when_each_calldata_size_is_zero() {
    val mockWeb3jClient = mock<ExtendedWeb3J> {
      on { ethGetBlockSizeByNumber(any()) } doReturn SafeFuture.completedFuture(BigInteger.ZERO)
    }
    val l2CalldataSizeAccumulator = L2CalldataSizeAccumulatorImpl(
      config = config,
      web3jClient = mockWeb3jClient,
    )

    val sumOfL2CalldataSize = l2CalldataSizeAccumulator.getSumOfL2CalldataSize(targetBlockNumber).get()

    (0..4).forEach {
      verify(mockWeb3jClient, times(1))
        .ethGetBlockSizeByNumber(eq(targetBlockNumber.toLong() - it))
    }

    assertThat(sumOfL2CalldataSize).isEqualTo(BigInteger.ZERO)
  }

  @Test
  fun test_getSumOfL2CalldataSize_throws_exception_when_ethGetBlockSizeByNumber_throws_exception() {
    val expectedException = RuntimeException("Failed for testing")
    val mockWeb3jClient = mock<ExtendedWeb3J> {
      on { ethGetBlockSizeByNumber(any()) } doReturn SafeFuture.failedFuture(expectedException)
    }
    val l2CalldataSizeAccumulator = L2CalldataSizeAccumulatorImpl(
      config = config,
      web3jClient = mockWeb3jClient,
    )

    assertThatThrownBy {
      l2CalldataSizeAccumulator.getSumOfL2CalldataSize(targetBlockNumber).get()
    }.hasCause(expectedException)
  }

  @Test
  fun test_getSumOfL2CalldataSize_returns_zero_when_target_block_number_is_less_than_calldataSizeBlockCount() {
    val mockWeb3jClient = mock<ExtendedWeb3J> {
      on { ethGetBlockSizeByNumber(any()) } doReturn SafeFuture.completedFuture(10540.toBigInteger())
    }
    val l2CalldataSizeAccumulator = L2CalldataSizeAccumulatorImpl(
      config = config,
      web3jClient = mockWeb3jClient,
    )

    val sumOfL2CalldataSize = l2CalldataSizeAccumulator.getSumOfL2CalldataSize(1UL).get()

    verify(mockWeb3jClient, times(0)).ethGetBlockSizeByNumber(any())

    assertThat(sumOfL2CalldataSize).isEqualTo(BigInteger.ZERO)
  }

  @Test
  fun test_getSumOfL2CalldataSize_returns_zero_when_calldataSizeBlockCount_is_zero() {
    val mockWeb3jClient = mock<ExtendedWeb3J> {
      on { ethGetBlockSizeByNumber(any()) } doReturn SafeFuture.completedFuture(10540.toBigInteger())
    }
    val l2CalldataSizeAccumulator = L2CalldataSizeAccumulatorImpl(
      config = L2CalldataSizeAccumulatorImpl.Config(
        blockSizeNonCalldataOverhead = 540u,
        calldataSizeBlockCount = 0u,
      ),
      web3jClient = mockWeb3jClient,
    )

    val sumOfL2CalldataSize = l2CalldataSizeAccumulator.getSumOfL2CalldataSize(targetBlockNumber).get()

    verify(mockWeb3jClient, times(0)).ethGetBlockSizeByNumber(any())

    assertThat(sumOfL2CalldataSize).isEqualTo(BigInteger.ZERO)
  }
}
