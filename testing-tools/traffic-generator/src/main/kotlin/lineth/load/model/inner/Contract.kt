package lineth.load.model.inner

import java.math.BigInteger

interface Contract {
  fun nbCalls(): Int
  fun gasLimit(): BigInteger
}
