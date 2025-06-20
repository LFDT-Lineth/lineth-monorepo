package net.consensys.linea.ethereum.gaspricing.staticcap

import linea.web3j.ExtendedWeb3J
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
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

  @Test
  fun test_getSumOfL2CalldataSize() {
    val mockWeb3jClient = mock<ExtendedWeb3J> {
      on { ethBlockNumber() } doReturn SafeFuture.completedFuture(100.toBigInteger())
      on { ethGetBlockSizeByNumber(any()) } doReturn SafeFuture.completedFuture(10540.toBigInteger())
    }
    val l2CalldataSizeAccumulator = L2CalldataSizeAccumulatorImpl(
      config = config,
      web3jClient = mockWeb3jClient,
    )

    val sumOfL2CalldataSize = l2CalldataSizeAccumulator.getSumOfL2CalldataSize().get()

    (0..4).forEach {
      verify(mockWeb3jClient, times(1))
        .ethGetBlockSizeByNumber(eq(100L - it))
    }

    val expectedCalldataSize = (10540 - 540) * 5
    assertThat(sumOfL2CalldataSize).isEqualTo(expectedCalldataSize.toBigInteger())
  }

  @Test
  fun test_getSumOfL2CalldataSize_for_each_calldata_size_at_zero() {
    val mockWeb3jClient = mock<ExtendedWeb3J> {
      on { ethBlockNumber() } doReturn SafeFuture.completedFuture(100.toBigInteger())
      on { ethGetBlockSizeByNumber(any()) } doReturn SafeFuture.completedFuture(BigInteger.ZERO)
    }
    val l2CalldataSizeAccumulator = L2CalldataSizeAccumulatorImpl(
      config = config,
      web3jClient = mockWeb3jClient,
    )

    val sumOfL2CalldataSize = l2CalldataSizeAccumulator.getSumOfL2CalldataSize().get()

    (0..4).forEach {
      verify(mockWeb3jClient, times(1))
        .ethGetBlockSizeByNumber(eq(100L - it))
    }

    assertThat(sumOfL2CalldataSize).isEqualTo(BigInteger.ZERO)
  }

  @Test
  fun test_getSumOfL2CalldataSize_for_exception() {
    val mockWeb3jClient = mock<ExtendedWeb3J> {
      on { ethBlockNumber() } doReturn SafeFuture.failedFuture(RuntimeException("Failed for testing"))
    }
    val l2CalldataSizeAccumulator = L2CalldataSizeAccumulatorImpl(
      config = config,
      web3jClient = mockWeb3jClient,
    )

    assertThrows<Exception> {
      l2CalldataSizeAccumulator.getSumOfL2CalldataSize().get()
    }
  }

  @Test
  fun test_getSumOfL2CalldataSize_when_ethBlockNumber_is_less_than_calldataSizeBlockCount() {
    val mockWeb3jClient = mock<ExtendedWeb3J> {
      on { ethBlockNumber() } doReturn SafeFuture.completedFuture(BigInteger.ONE)
      on { ethGetBlockSizeByNumber(any()) } doReturn SafeFuture.completedFuture(10540.toBigInteger())
    }
    val l2CalldataSizeAccumulator = L2CalldataSizeAccumulatorImpl(
      config = config,
      web3jClient = mockWeb3jClient,
    )

    val sumOfL2CalldataSize = l2CalldataSizeAccumulator.getSumOfL2CalldataSize().get()

    verify(mockWeb3jClient, times(0)).ethGetBlockSizeByNumber(any())

    assertThat(sumOfL2CalldataSize).isEqualTo(BigInteger.ZERO)
  }

  @Test
  fun test_getSumOfL2CalldataSize_if_calldataSizeBlockCount_is_zero() {
    val mockWeb3jClient = mock<ExtendedWeb3J> {
      on { ethBlockNumber() } doReturn SafeFuture.completedFuture(BigInteger.ONE)
      on { ethGetBlockSizeByNumber(any()) } doReturn SafeFuture.completedFuture(10540.toBigInteger())
    }
    val l2CalldataSizeAccumulator = L2CalldataSizeAccumulatorImpl(
      config = L2CalldataSizeAccumulatorImpl.Config(
        blockSizeNonCalldataOverhead = 540u,
        calldataSizeBlockCount = 0u,
      ),
      web3jClient = mockWeb3jClient,
    )

    val sumOfL2CalldataSize = l2CalldataSizeAccumulator.getSumOfL2CalldataSize().get()

    verify(mockWeb3jClient, times(0)).ethGetBlockSizeByNumber(any())

    assertThat(sumOfL2CalldataSize).isEqualTo(BigInteger.ZERO)
  }
}
