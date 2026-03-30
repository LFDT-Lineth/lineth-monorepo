package net.consensys.zkevm.ethereum.coordination

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class DynamicBlockNumberSetTest {
  @Test
  fun `set exposes initial block numbers in ascending order when nothing else registered`() {
    val store = DynamicBlockNumberSet(initialBlockNumbers = listOf(10uL, 20uL))
    assertThat(store).containsExactly(10uL, 20uL)
  }

  @Test
  fun `initial blocks numbers are sorted when provided out of order`() {
    val store = DynamicBlockNumberSet(initialBlockNumbers = listOf(30uL, 10uL, 20uL))
    assertThat(store).containsExactly(10uL, 20uL, 30uL)
  }

  @Test
  fun `addBlockNumber deduplicates and iteration is ascending`() {
    val store = DynamicBlockNumberSet(initialBlockNumbers = listOf(10uL, 20uL))
    store.addBlockNumber(15uL)
    store.addBlockNumber(15uL)
    store.addBlockNumber(5uL)
    assertThat(store).containsExactly(5uL, 10uL, 15uL, 20uL)
  }

  @Test
  fun `default store is empty`() {
    val store = DynamicBlockNumberSet()
    assertThat(store).isEmpty()
  }

  @Test
  fun `removeBlockNumber removes present value and returns true`() {
    val store = DynamicBlockNumberSet(initialBlockNumbers = listOf(10uL, 20uL))
    assertThat(store.removeBlockNumber(10uL)).isTrue()
    assertThat(store).containsExactly(20uL)
  }

  @Test
  fun `removeBlockNumber returns false when value absent`() {
    val store = DynamicBlockNumberSet(initialBlockNumbers = listOf(10uL))
    assertThat(store.removeBlockNumber(99uL)).isFalse()
    assertThat(store).containsExactly(10uL)
  }
}
