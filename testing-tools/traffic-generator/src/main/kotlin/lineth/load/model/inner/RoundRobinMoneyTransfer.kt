package lineth.load.model.inner

import lineth.load.model.EthConnection
import java.math.BigInteger

class RoundRobinMoneyTransfer(val wallet: String, val nbTransfers: Int, val nbWallets: Int) : Scenario {

  companion object {
    val valueToTransfer: BigInteger = BigInteger.valueOf(1000)
  }

  override fun wallet(): String {
    return wallet
  }

  override fun gasLimit(): BigInteger {
    return EthConnection.SIMPLE_TX_PRICE.add(BigInteger.valueOf(1000L))
  }
}
