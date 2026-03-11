package linea.kotlin

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Nested
import org.junit.jupiter.api.Test

class ListExtensionsTest {

  @Nested
  inner class ByteArrayListEquals {
    @Test
    fun `returns true for both empty lists`() {
      assertThat(emptyList<ByteArray>().byteArrayListEquals(emptyList())).isTrue()
    }

    @Test
    fun `returns true for lists with same content`() {
      val list1 = listOf(byteArrayOf(1, 2, 3), byteArrayOf(4, 5))
      val list2 = listOf(byteArrayOf(1, 2, 3), byteArrayOf(4, 5))
      assertThat(list1.byteArrayListEquals(list2)).isTrue()
    }

    @Test
    fun `returns false for lists with different sizes`() {
      val list1 = listOf(byteArrayOf(1, 2, 3))
      val list2 = listOf(byteArrayOf(1, 2, 3), byteArrayOf(4, 5))
      assertThat(list1.byteArrayListEquals(list2)).isFalse()
    }

    @Test
    fun `returns false for lists with different content`() {
      val list1 = listOf(byteArrayOf(1, 2, 3), byteArrayOf(4, 5))
      val list2 = listOf(byteArrayOf(1, 2, 3), byteArrayOf(4, 6))
      assertThat(list1.byteArrayListEquals(list2)).isFalse()
    }

    @Test
    fun `returns false for lists with same elements in different order`() {
      val list1 = listOf(byteArrayOf(1, 2), byteArrayOf(3, 4))
      val list2 = listOf(byteArrayOf(3, 4), byteArrayOf(1, 2))
      assertThat(list1.byteArrayListEquals(list2)).isFalse()
    }

    @Test
    fun `returns true for single element lists with same content`() {
      val list1 = listOf(byteArrayOf(0xFF.toByte()))
      val list2 = listOf(byteArrayOf(0xFF.toByte()))
      assertThat(list1.byteArrayListEquals(list2)).isTrue()
    }
  }

  @Nested
  inner class ByteArrayListHashCode {
    @Test
    fun `returns same hash code for lists with same content`() {
      val list1 = listOf(byteArrayOf(1, 2, 3), byteArrayOf(4, 5))
      val list2 = listOf(byteArrayOf(1, 2, 3), byteArrayOf(4, 5))
      assertThat(list1.byteArrayListHashCode()).isEqualTo(list2.byteArrayListHashCode())
    }

    @Test
    fun `returns different hash codes for lists with different content`() {
      val list1 = listOf(byteArrayOf(1, 2, 3), byteArrayOf(4, 5))
      val list2 = listOf(byteArrayOf(1, 2, 3), byteArrayOf(4, 6))
      assertThat(list1.byteArrayListHashCode()).isNotEqualTo(list2.byteArrayListHashCode())
    }

    @Test
    fun `returns different hash codes for lists with different order`() {
      val list1 = listOf(byteArrayOf(1, 2), byteArrayOf(3, 4))
      val list2 = listOf(byteArrayOf(3, 4), byteArrayOf(1, 2))
      assertThat(list1.byteArrayListHashCode()).isNotEqualTo(list2.byteArrayListHashCode())
    }

    @Test
    fun `returns same hash code for both empty lists`() {
      assertThat(emptyList<ByteArray>().byteArrayListHashCode())
        .isEqualTo(emptyList<ByteArray>().byteArrayListHashCode())
    }

    @Test
    fun `is consistent with byteArrayListEquals`() {
      val list1 = listOf(byteArrayOf(10, 20), byteArrayOf(30, 40, 50))
      val list2 = listOf(byteArrayOf(10, 20), byteArrayOf(30, 40, 50))
      assertThat(list1.byteArrayListEquals(list2)).isTrue()
      assertThat(list1.byteArrayListHashCode()).isEqualTo(list2.byteArrayListHashCode())
    }
  }
}
