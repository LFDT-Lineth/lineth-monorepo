package net.consensys.zkevm.load

import net.consensys.zkevm.load.model.DummyEthConnection
import net.consensys.zkevm.load.model.DummyWalletL1State
import net.consensys.zkevm.load.model.ExpectedOutcome
import net.consensys.zkevm.load.model.Wallet
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Test
import org.web3j.crypto.Keys
import java.math.BigInteger
import kotlin.test.assertFailsWith

class WalletsFundingTest {

  val keyPair = Keys.createEcKeyPair()
  val sourceWallet = Wallet(keyPair.privateKey.toString(), -1, BigInteger.valueOf(1000))

  private val eth = DummyEthConnection()
  private val walletsFunding: WalletsFunding = WalletsFunding(eth, sourceWallet)

  @Test
  fun generateTxWithRandomPayload() {
    val wallet1 = Wallet(keyPair.privateKey.toString(), 1, BigInteger.valueOf(1000))
    val wallet2 = Wallet(keyPair.privateKey.toString(), 2, BigInteger.valueOf(1000))

    val txs = walletsFunding.generateTxWithRandomPayload(
      mapOf(-1 to sourceWallet, 1 to wallet1, 2 to wallet2),
      100,
      1944,
      1,
    )
    assertEquals(3, txs.size)
    assertEquals(1, txs[wallet1]?.size)
    assertEquals(1, txs[wallet1]?.get(0)?.walletId)
    assertEquals(200, txs[wallet1]?.get(0)?.ethSendTransactionRequest?.params?.get(0).toString().length)
    assertEquals(null, txs[wallet1]?.get(0)?.hash) // not set as not send yet
    assertEquals(ExpectedOutcome.SUCCESS, txs[wallet1]?.get(0)?.expectedOutcome) // move to NOT_EXECUTED ?
    assertEquals(BigInteger.valueOf(1000), txs[wallet1]?.get(0)?.nonce)
    assertEquals(BigInteger.valueOf(1001), sourceWallet.theoreticalNonceValue)
  }

  @Test
  fun initializeWallets() {
    eth.wallets[sourceWallet.encodedAddress()] = DummyWalletL1State(sourceWallet, BigInteger.TEN)

    val txs = walletsFunding.initializeWallets(
      mapOf(-1 to sourceWallet),
      2,
      sourceWallet,
      1944,
      BigInteger.valueOf(2500),
      BigInteger.valueOf(2500),
      BigInteger.valueOf(1),
    )

    assertEquals(1, txs.size)
    assertEquals(1, txs.get(sourceWallet)?.size)
  }

  @Test
  fun generateTransactions() {
    val wallet1 = Wallet(keyPair.privateKey.toString(), 1, BigInteger.valueOf(1000))
    val wallet2 = Wallet(keyPair.privateKey.toString(), 2, BigInteger.valueOf(1000))

    val txs = walletsFunding.generateTransactions(
      mapOf(0 to wallet1, 1 to wallet2),
      BigInteger.TWO,
      2,
      1944,
    )

    assertEquals(2, txs.size)
    assertEquals(2, txs.get(wallet1)?.size)
  }

  @Test
  fun timeout_waitForTransactions() {
    assertFailsWith<InterruptedException>(
      "should time out",
      block = { walletsFunding.waitForTransactions(mapOf(sourceWallet to BigInteger.ONE), 1L) },
    )
  }

  @Test
  fun completed_waitForTransactions() {
    val wallet1 = Wallet(keyPair.privateKey.toString(), 1, BigInteger.valueOf(1000))
    eth.wallets.put(wallet1.encodedAddress(), DummyWalletL1State(wallet1, BigInteger.TEN))

    wallet1.initialNonce = BigInteger.TWO
    walletsFunding.waitForTransactions(mapOf(wallet1 to BigInteger.ONE), 1L)
  }
}
