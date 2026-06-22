package net.consensys.zkevm.load.model.inner

import java.math.BigInteger

class CreateContract(val name: String, val byteCode: String, val gasLimit: BigInteger) : Contract {
  override fun nbCalls(): Int {
    return 1
  }

  override fun gasLimit(): BigInteger {
    return gasLimit
  }
}
