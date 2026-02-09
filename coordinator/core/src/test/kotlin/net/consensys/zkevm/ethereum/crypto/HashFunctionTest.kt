package net.consensys.zkevm.ethereum.crypto

import org.junit.jupiter.api.Assertions.assertArrayEquals
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Disabled
import org.junit.jupiter.api.Test
import java.security.MessageDigest
import kotlin.concurrent.atomics.AtomicBoolean
import kotlin.concurrent.atomics.ExperimentalAtomicApi
import kotlin.random.Random

@OptIn(ExperimentalAtomicApi::class)
class HashFunctionTest {
  @Test
  fun `test single MessageDigest instance does not produce rolling hashes`() {
    val digester = MessageDigest.getInstance("SHA-256")
    for (i in 1..10) {
      val randomBytes = Random.nextBytes(32)
      assertArrayEquals(
        digester.digest(randomBytes),
        MessageDigest.getInstance("SHA-256").digest(randomBytes),
      )
    }
  }

  @Test
  fun `test Sha256HashFunction produces correct hash`() {
    val hashFunction = Sha256HashFunction()
    for (i in 1..10) {
      val randomBytes = Random.nextBytes(32)
      assertArrayEquals(
        hashFunction.hash(randomBytes),
        MessageDigest.getInstance("SHA-256").digest(randomBytes),
      )
    }
  }

  @Test
  fun `test Sha256HashFunction under concurrent calls produces correct hash`() {
    val hashFunction = Sha256HashFunction()
    val anyFailures = AtomicBoolean(false)
    val threads = List(10) {
      Thread {
        for (i in 1..1000) {
          try {
            val randomBytes = Random.nextBytes(32)
            val hashResult = hashFunction.hash(randomBytes)
            val expectedHash = MessageDigest.getInstance("SHA-256").digest(randomBytes)
            if (!hashResult.contentEquals(expectedHash)) {
              anyFailures.store(true)
            }
          } catch (e: Exception) {
            anyFailures.store(true)
          }
        }
      }
    }
    threads.forEach { it.start() }
    threads.forEach { it.join() }
    assertFalse(anyFailures.load()) { "Concurrent calls to Sha256HashFunction produced incorrect hashes" }
  }

  @Test
  @Disabled(
    "This test demonstrates that using a single MessageDigest instance under concurrent calls " +
      "produces incorrect hashes, which is why Sha256HashFunction creates a new MessageDigest instance for each call.",
  )
  fun `test Sha256HashFunction with single MessageDigest under concurrent calls produces wrong hash`() {
    val hashFunction = object : HashFunction {
      private val digester = MessageDigest.getInstance("SHA-256")
      override fun hash(bytes: ByteArray): ByteArray {
        return digester.digest(bytes)
      }
    }
    val anyFailures = AtomicBoolean(false)
    val threads = List(10) {
      Thread {
        for (i in 1..1000) {
          try {
            val randomBytes = Random.nextBytes(32)
            val hashResult = hashFunction.hash(randomBytes)
            val expectedHash = MessageDigest.getInstance("SHA-256").digest(randomBytes)
            if (!hashResult.contentEquals(expectedHash)) {
              anyFailures.store(true)
            }
          } catch (e: Exception) {
            anyFailures.store(true)
          }
        }
      }
    }
    threads.forEach { it.start() }
    threads.forEach { it.join() }
    assertTrue(anyFailures.load()) { "Concurrent calls to Sha256HashFunction produced incorrect hashes" }
  }
}
