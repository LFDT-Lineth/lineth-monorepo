package lineth.load.model.inner

import java.math.BigInteger

interface Scenario {
  fun wallet(): String

  fun gasLimit(): BigInteger
}
