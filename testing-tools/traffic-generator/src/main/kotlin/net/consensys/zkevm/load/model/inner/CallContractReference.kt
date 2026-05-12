package net.consensys.zkevm.load.model.inner

import java.math.BigInteger

class CallContractReference(val contractName: String, val methodAndParameters: MethodAndParameter) : Contract {
  override fun nbCalls(): Int {
    return methodAndParameters.nbOfTimes
  }

  override fun gasLimit(): BigInteger {
    return BigInteger.valueOf(930000L)
  }
}
