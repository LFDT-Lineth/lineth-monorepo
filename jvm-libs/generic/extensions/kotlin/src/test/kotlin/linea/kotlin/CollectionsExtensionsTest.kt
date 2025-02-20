package linea.kotlin

import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test

class CollectionsExtensionsTest {
  @Test
  fun testIsSortedBy() {
    assertTrue(listOf(1, 2, 4, 5, 6).isSortedBy { it })
    assertTrue(listOf(1).isSortedBy { it })
    assertTrue(emptyList<Int>().isSortedBy { it })
    assertFalse(listOf(1, 2, 7, 6).isSortedBy { it })
  }
}
