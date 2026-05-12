package net.consensys.zkevm.load.model.inner

import java.math.BigInteger

class UnderPricedTransaction(val wallet: String, val nbTransfers: Int) : Scenario {
  override fun wallet(): String {
    return wallet
  }

  override fun gasLimit(): BigInteger {
    return BigInteger.valueOf(19000L)
  }
}
