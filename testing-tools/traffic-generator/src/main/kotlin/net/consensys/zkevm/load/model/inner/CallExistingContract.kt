package net.consensys.zkevm.load.model.inner

import java.math.BigInteger

class CallExistingContract(val contractAddress: String, val methodAndParameters: MethodAndParameter) : Contract {
  override fun nbCalls(): Int {
    return methodAndParameters.nbOfTimes
  }

  override fun gasLimit(): BigInteger {
    return methodAndParameters.gasLimit()
  }
}
