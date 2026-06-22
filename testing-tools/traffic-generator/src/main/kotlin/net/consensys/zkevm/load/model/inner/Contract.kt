package net.consensys.zkevm.load.model.inner

import java.math.BigInteger

interface Contract {
  fun nbCalls(): Int
  fun gasLimit(): BigInteger
}
