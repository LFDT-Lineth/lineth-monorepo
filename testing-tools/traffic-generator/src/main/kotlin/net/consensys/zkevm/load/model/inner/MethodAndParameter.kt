package net.consensys.zkevm.load.model.inner

import java.math.BigInteger

abstract class MethodAndParameter(val nbOfTimes: Int) {
  abstract fun gasLimit(): BigInteger
}
