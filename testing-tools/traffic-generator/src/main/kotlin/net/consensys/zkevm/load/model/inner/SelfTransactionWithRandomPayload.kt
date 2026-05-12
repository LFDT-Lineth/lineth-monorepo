package net.consensys.zkevm.load.model.inner

import java.math.BigInteger

class SelfTransactionWithRandomPayload(
  val wallet: String,
  val nbWallets: Int,
  val nbTransfers: Int,
  val payloadSize: Int,
  val price: BigInteger,
) : Scenario {
  override fun wallet(): String {
    return wallet
  }

  override fun gasLimit(): BigInteger {
    return price.add(BigInteger.valueOf((payloadSize * 16).toLong()))
  }
}
