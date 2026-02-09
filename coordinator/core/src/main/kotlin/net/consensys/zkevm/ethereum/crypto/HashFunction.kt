package net.consensys.zkevm.ethereum.crypto

import java.security.MessageDigest

fun interface HashFunction {
  fun hash(bytes: ByteArray): ByteArray
}

class Sha256HashFunction : HashFunction {
  override fun hash(bytes: ByteArray): ByteArray {
    return MessageDigest.getInstance("SHA-256").digest(bytes)
  }
}
