package net.consensys.zkevm.ethereum.crypto

import java.security.MessageDigest

fun interface HashFunction {
  fun hash(bytes: ByteArray): ByteArray
}

class Sha256HashFunction : HashFunction {
  private val digest: MessageDigest = MessageDigest.getInstance("SHA-256")

  override fun hash(bytes: ByteArray): ByteArray {
    return digest.digest(bytes)
  }
}
