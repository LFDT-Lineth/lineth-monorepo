package net.consensys.zkevm.load.model.inner

import java.math.BigInteger

class GenericCall(nbOfTimes: Int, val methodName: String, val price: Long, val parameters: List<Parameter>) :
  MethodAndParameter(nbOfTimes) {
  override fun gasLimit(): BigInteger {
    return BigInteger.valueOf(price)
  }
}
