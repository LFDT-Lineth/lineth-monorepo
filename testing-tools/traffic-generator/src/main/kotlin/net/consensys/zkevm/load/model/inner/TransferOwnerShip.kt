package net.consensys.zkevm.load.model.inner

import java.math.BigInteger

class TransferOwnerShip(nbOfTimes: Int, val destinationAddress: String) : MethodAndParameter(nbOfTimes) {
  override fun gasLimit(): BigInteger {
    return BigInteger.valueOf(930000L)
  }
}
