package linea.web3j.gas

import linea.kotlin.gwei
import linea.kotlin.toBigInteger
import org.web3j.protocol.core.methods.request.Transaction
import java.math.BigInteger

// this class is mainly intended to be used for testing purposes
class StaticGasProvider(
  private val _chainId: Long,
  // setting default high values because
  // tests suite sends loads of Tx and blobs, causes spikes in gas prices
  private val maxFeePerGas: ULong = 22uL.gwei,
  private val maxPriorityFeePerGas: ULong = 20uL.gwei,
  private val maxFeePerBlobGas: ULong = 1000uL.gwei,
  private val gasLimit: ULong = 30_000_000uL,
) : AtomicContractEIP1559GasProvider, EIP4844GasProvider {
  override fun getEIP1559GasFees(): EIP1559GasFees {
    return EIP1559GasFees(maxPriorityFeePerGas, maxFeePerGas)
  }

  @Deprecated("Deprecated in Java")
  override fun getGasPrice(): BigInteger {
    return maxFeePerGas.toBigInteger()
  }

  override fun getGasLimit(transaction: Transaction): BigInteger {
    return gasLimit.toBigInteger()
  }

  @Deprecated("Deprecated in Java")
  override fun getGasLimit(): BigInteger {
    return gasLimit.toBigInteger()
  }

  override fun getChainId(): Long {
    return _chainId
  }

  override fun getMaxFeePerGas(): BigInteger {
    return maxFeePerGas.toBigInteger()
  }

  override fun getMaxPriorityFeePerGas(): BigInteger {
    return maxPriorityFeePerGas.toBigInteger()
  }

  override fun getEIP4844GasFees(): EIP4844GasFees {
    return EIP4844GasFees(getEIP1559GasFees(), maxFeePerBlobGas)
  }
}
