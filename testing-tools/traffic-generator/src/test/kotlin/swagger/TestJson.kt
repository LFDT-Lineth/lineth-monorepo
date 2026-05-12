package net.consensys.zkevm.load.model.swagger

import net.consensys.zkevm.load.model.JSON
import net.consensys.zkevm.load.model.Wallet
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import java.math.BigInteger

class TestJson {
  companion object {
    private val PRIVATE_KEY = "11".repeat(32)
  }

  @Test
  fun toJsonWalletRedactsSecrets() {
    // Arrange
    val wallet = Wallet(PRIVATE_KEY, 0, BigInteger.ZERO)

    // Act
    val walletJson = JSON.createGson().create().toJson(wallet)

    // Assert
    assertEquals("""{"id":0,"initialNonce":0,"address":"${wallet.address}"}""", walletJson)
    assertFalse(walletJson.contains(wallet.privateKey))
    assertFalse(walletJson.contains("credentials"))
  }

  @Test
  fun mapSerializationDoesNotLeakWalletSecrets() {
    // Arrange
    val wallet = Wallet(PRIVATE_KEY, 0, BigInteger.ZERO)

    // Act
    val reportJson = JSON.createGson().create().toJson(mapOf(wallet to listOf(1)))

    // Assert
    assertFalse(reportJson.contains(wallet.privateKey))
    assertFalse(reportJson.contains("credentials"))
    assertTrue(reportJson.contains(wallet.address))
  }
}
