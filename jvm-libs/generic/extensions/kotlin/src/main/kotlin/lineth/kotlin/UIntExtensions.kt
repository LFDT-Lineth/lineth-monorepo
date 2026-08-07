package lineth.kotlin

fun UInt.minusCoercingUnderflow(other: UInt): UInt {
  return if (this > other) {
    this - other
  } else {
    0U
  }
}
