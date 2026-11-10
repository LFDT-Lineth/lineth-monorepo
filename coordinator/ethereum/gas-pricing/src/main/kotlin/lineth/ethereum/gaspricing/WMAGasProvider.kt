package lineth.ethereum.gaspricing

import linea.kotlin.toBigInteger
import linea.web3j.gas.AtomicContractEIP1559GasProvider
import linea.web3j.gas.EIP1559GasFees
import linea.web3j.gas.EIP4844GasFees
import linea.web3j.gas.EIP4844GasProvider
import org.web3j.protocol.core.methods.request.Transaction
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import kotlin.math.min

class WMAGasProvider(
  private val chainId: Long,
  private val feesFetcher: FeesFetcher,
  private val priorityFeeCalculator: FeesCalculator,
  private val config: Config,
) :
  AtomicContractEIP1559GasProvider, EIP4844GasProvider {
  data class Config(
    val gasLimit: ULong,
    val maxFeePerGasCap: ULong,
    val maxFeePerBlobGasCap: ULong,
    val maxPriorityFeePerGasCap: ULong,
  )

  private fun getRecentFeesAsync(): SafeFuture<EIP4844GasFees> {
    return feesFetcher.getL1EthGasPriceData().thenApply { feesData ->
      val maxPriorityFeePerGas =
        priorityFeeCalculator.calculateFees(feesData)
          .coerceAtMost(config.maxPriorityFeePerGasCap.toDouble())
      val maxFeePerGas = (feesData.baseFeePerGas.last().toDouble() * 2) + maxPriorityFeePerGas
      val maxFeePerBlobGas = config.maxFeePerBlobGasCap
      EIP4844GasFees(
        EIP1559GasFees(
          maxPriorityFeePerGas.toULong(),
          maxFeePerGas.toULong(),
        ),
        maxFeePerBlobGas,
      )
    }
  }

  @Deprecated("Deprecated in Java")
  override fun getGasPrice(): BigInteger {
    throw NotImplementedError("EIP1559GasProvider only implements EIP1559 specific methods")
  }

  override fun getGasLimit(transaction: Transaction?): BigInteger {
    return config.gasLimit.toBigInteger()
  }

  @Deprecated("Deprecated in Java")
  override fun getGasLimit(): BigInteger {
    return config.gasLimit.toBigInteger()
  }

  override fun getChainId(): Long {
    return chainId
  }

  // Synchronous getters block until fees are fetched. They are kept only for the web3j
  // ContractGasProvider contract and blocking callers; async code must use the *Async variants.
  override fun getMaxFeePerGas(): BigInteger {
    return getEIP1559GasFees().maxFeePerGas.toBigInteger()
  }

  override fun getMaxPriorityFeePerGas(): BigInteger {
    return getEIP1559GasFees().maxPriorityFeePerGas.toBigInteger()
  }

  override fun getEIP1559GasFees(): EIP1559GasFees {
    return getEIP1559GasFeesAsync().get()
  }

  override fun getEIP4844GasFees(): EIP4844GasFees {
    return getEIP4844GasFeesAsync().get()
  }

  override fun getEIP1559GasFeesAsync(): SafeFuture<EIP1559GasFees> {
    return getRecentFeesAsync().thenApply {
      EIP1559GasFees(
        maxPriorityFeePerGas = min(it.eip1559GasFees.maxPriorityFeePerGas, config.maxFeePerGasCap),
        maxFeePerGas = min(it.eip1559GasFees.maxFeePerGas, config.maxFeePerGasCap),
      )
    }
  }

  override fun getEIP4844GasFeesAsync(): SafeFuture<EIP4844GasFees> {
    return getRecentFeesAsync().thenApply {
      EIP4844GasFees(
        eip1559GasFees = EIP1559GasFees(
          maxPriorityFeePerGas = min(it.eip1559GasFees.maxPriorityFeePerGas, config.maxFeePerGasCap),
          maxFeePerGas = min(it.eip1559GasFees.maxFeePerGas, config.maxFeePerGasCap),
        ),
        maxFeePerBlobGas = min(it.maxFeePerBlobGas, config.maxFeePerBlobGasCap),
      )
    }
  }
}
