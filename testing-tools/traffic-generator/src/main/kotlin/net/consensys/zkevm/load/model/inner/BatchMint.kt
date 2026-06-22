package net.consensys.zkevm.load.model.inner

import java.math.BigInteger

class BatchMint(nbOfTimes: Int, val address: List<String>, val amount: Int) : MethodAndParameter(nbOfTimes) {
  override fun gasLimit(): BigInteger {
    return BigInteger.valueOf(930000L)
  }
}
