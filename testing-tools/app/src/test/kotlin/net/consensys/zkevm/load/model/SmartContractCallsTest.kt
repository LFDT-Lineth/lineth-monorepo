package net.consensys.zkevm.load.model

import net.consensys.zkevm.load.model.inner.SimpleParameter
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Test
import org.web3j.crypto.Keys
import java.math.BigInteger

private const val DEFAULT_ADDRESS = "0x1d80c1698946ee65b6A6B78F468B47f6bd4516f0"

class SmartContractCallsTest {

  private val MINT_ENCODED =
    "0x40c10f190000000000000000000000001d80c1698946ee65b6a6b78f468b47f6bd4516f" +
      "00000000000000000000000000000000000000000000000000000000000000001"
  private val smartContractCalls: SmartContractCalls = SmartContractCalls(DummyEthConnection())

  @Test
  fun genericCall() {
    val callFunction = smartContractCalls.genericCall(
      "mint",
      listOf(
        SimpleParameter(DEFAULT_ADDRESS, "Address"),
        SimpleParameter("1", "Uint256"),
      ),
    )
    assertEquals(MINT_ENCODED, callFunction)
  }

  @Test
  fun mint() {
    val callFunction = smartContractCalls.mint(DEFAULT_ADDRESS, 1)
    assertEquals(MINT_ENCODED, callFunction)
  }

  @Test
  fun getRequests() {
    val keyPair = Keys.createEcKeyPair()
    val sourceWallet = Wallet(keyPair.privateKey.toString(), -1, BigInteger.valueOf(1000))
    val requests = smartContractCalls.getRequests(
      "0x9d97700664141F25638463C5F1A024Ea24D902f5",
      sourceWallet,
      MINT_ENCODED,
      10,
      1944,
    )
    assertEquals(1, requests.size)
    assertEquals(10, requests.get(sourceWallet)?.size)
    assertEquals(BigInteger.valueOf(1010), sourceWallet.theoreticalNonceValue)
  }
}
