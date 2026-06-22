package net.consensys.zkevm.load.model.inner

import net.consensys.zkevm.load.model.Wallet
import java.lang.UnsupportedOperationException
import java.math.BigInteger

class ContractCall(val wallet: String, val contract: Contract) : Scenario {
  override fun wallet(): String {
    return wallet
  }

  override fun gasLimit(): BigInteger {
    return contract.gasLimit()
  }

  fun wallet(sourceWallet: Wallet, walletMap: Map<Int, Wallet>): Wallet {
    if (wallet.equals(SOURCE_WALLET)) {
      return sourceWallet
    } else if (wallet.equals(NEW)) {
      return walletMap.get(0)!!
    }
    throw UnsupportedOperationException("Not implemented")
  }
}
