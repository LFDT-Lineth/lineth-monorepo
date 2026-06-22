package net.consensys.zkevm.load.model.inner

import java.math.BigInteger

interface Scenario {
  fun wallet(): String

  fun gasLimit(): BigInteger
}
