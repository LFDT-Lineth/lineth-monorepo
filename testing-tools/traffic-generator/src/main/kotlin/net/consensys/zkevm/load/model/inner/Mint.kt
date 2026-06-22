package net.consensys.zkevm.load.model.inner

import java.math.BigInteger

class Mint(nbOfTimes: Int, val address: String, val amount: Int) : MethodAndParameter(nbOfTimes) {
  override fun gasLimit(): BigInteger {
    return BigInteger.valueOf(930000L)
  }
}
