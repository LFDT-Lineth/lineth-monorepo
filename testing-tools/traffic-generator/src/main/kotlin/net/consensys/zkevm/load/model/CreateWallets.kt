package net.consensys.zkevm.load.model

import org.web3j.crypto.Keys
import java.math.BigInteger
import java.security.InvalidAlgorithmParameterException
import java.security.NoSuchAlgorithmException
import java.security.NoSuchProviderException

object CreateWallets {
  @JvmStatic
  @Throws(InvalidAlgorithmParameterException::class, NoSuchAlgorithmException::class, NoSuchProviderException::class)
  fun createWallets(numberOfWallets: Int): Map<Int, Wallet> {
    val wallets: MutableMap<Int, Wallet> = HashMap()
    for (walletId in 0 until numberOfWallets) {
      val keyPair = Keys.createEcKeyPair()
      wallets[walletId] = Wallet(keyPair.privateKey.toString(16), walletId, BigInteger.ZERO)
    }
    return wallets
  }
}
