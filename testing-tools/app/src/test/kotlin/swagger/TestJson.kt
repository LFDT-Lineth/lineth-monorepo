package net.consensys.zkevm.load.model.swagger

import net.consensys.zkevm.load.model.JSON
import net.consensys.zkevm.load.model.Wallet
import org.junit.jupiter.api.Assertions
import org.junit.jupiter.api.Test
import org.web3j.crypto.Keys
import java.math.BigInteger

class TestJson {

  @Test
  fun testToJsonWallet() {
    val keyPair = Keys.createEcKeyPair()
    val wallet = Wallet(keyPair.privateKey.toString(16), 0, BigInteger.ZERO)
    Assertions.assertEquals(
      """
      {"privateKey":"${wallet.privateKey}","credentials":{"ecKeyPair":{"privateKey":${wallet.credentials.ecKeyPair.privateKey},"publicKey":${wallet.credentials.ecKeyPair.publicKey}},"address":"${wallet.credentials.address}"},"id":0,"initialNonce":0,"address":"${wallet.address}"}
      """.trimIndent(),
      JSON.createGson().create().toJson(wallet),
    )
  }
}
