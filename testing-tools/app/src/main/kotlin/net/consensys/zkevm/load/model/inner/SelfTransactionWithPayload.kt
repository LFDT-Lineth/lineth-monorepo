package net.consensys.zkevm.load.model.inner

import java.math.BigInteger

class SelfTransactionWithPayload(
  val wallet: String,
  val nbWallets: Int,
  val nbTransfers: Int,
  val payload: String,
  val price: BigInteger,
) : Scenario {
  override fun wallet(): String {
    return wallet
  }

  override fun gasLimit(): BigInteger {
    return price.add(BigInteger.valueOf((payload.length * 16).toLong()))
  }
}
