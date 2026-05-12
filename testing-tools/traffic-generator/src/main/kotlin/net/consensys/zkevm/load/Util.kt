package net.consensys.zkevm.load

import org.apache.tuweni.bytes.Bytes
import org.web3j.utils.Numeric

object Util {
  fun generateRandomPayloadOfSize(payloadSize: Int): String {
    val bytes = Bytes.random(payloadSize)
    return Numeric.toHexString(bytes.toArray())
  }
}
