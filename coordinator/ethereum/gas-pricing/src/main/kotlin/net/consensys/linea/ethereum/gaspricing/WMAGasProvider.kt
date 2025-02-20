package net.consensys.linea.ethereum.gaspricing

import linea.kotlin.toBigInteger
import net.consensys.linea.web3j.AtomicContractEIP1559GasProvider
import net.consensys.linea.web3j.EIP1559GasFees
import net.consensys.linea.web3j.EIP4844GasFees
import net.consensys.linea.web3j.EIP4844GasProvider
import java.math.BigInteger
import kotlin.math.min

class WMAGasProvider(
  private val chainId: Long,
  private val feesFetcher: FeesFetcher,
  private val priorityFeeCalculator: FeesCalculator,
  private val config: Config
) :
  AtomicContractEIP1559GasProvider, EIP4844GasProvider {
  data class Config(
    val gasLimit: ULong,
    val maxFeePerGasCap: ULong,
    val maxFeePerBlobGasCap: ULong,
    val maxPriorityFeePerGasCap: ULong
  )

  private fun getRecentFees(): EIP4844GasFees {
    return feesFetcher.getL1EthGasPriceData().thenApply {
        feesData ->
      val maxPriorityFeePerGas = priorityFeeCalculator.calculateFees(feesData)
        .coerceAtMost(config.maxPriorityFeePerGasCap.toDouble())
      val maxFeePerGas = (feesData.baseFeePerGas.last().toDouble() * 2) + maxPriorityFeePerGas
      val maxFeePerBlobGas = config.maxFeePerBlobGasCap
      EIP4844GasFees(
        EIP1559GasFees(
          maxPriorityFeePerGas.toULong(),
          maxFeePerGas.toULong()
        ),
        maxFeePerBlobGas
      )
    }.get()
  }

  @Suppress("Deprecation")
  override fun getGasPrice(contractFunc: String?): BigInteger {
    return gasPrice
  }

  @Deprecated("Deprecated in Java")
  override fun getGasPrice(): BigInteger {
    throw NotImplementedError("EIP1559GasProvider only implements EIP1559 specific methods")
  }

  @Suppress("Deprecation")
  override fun getGasLimit(contractFunc: String?): BigInteger {
    return gasLimit
  }

  @Deprecated("Deprecated in Java")
  override fun getGasLimit(): BigInteger {
    return config.gasLimit.toBigInteger()
  }

  override fun isEIP1559Enabled(): Boolean {
    return true
  }

  override fun getChainId(): Long {
    return chainId
  }

  override fun getMaxFeePerGas(contractFunc: String?): BigInteger {
    return min(getRecentFees().eip1559GasFees.maxFeePerGas, config.maxFeePerGasCap).toBigInteger()
  }

  override fun getMaxPriorityFeePerGas(contractFunc: String?): BigInteger {
    return min(getRecentFees().eip1559GasFees.maxPriorityFeePerGas, config.maxFeePerGasCap).toBigInteger()
  }

  override fun getEIP1559GasFees(): EIP1559GasFees {
    return getRecentFees().run {
      EIP1559GasFees(
        maxPriorityFeePerGas = min(this.eip1559GasFees.maxPriorityFeePerGas, config.maxFeePerGasCap),
        maxFeePerGas = min(this.eip1559GasFees.maxFeePerGas, config.maxFeePerGasCap)
      )
    }
  }

  override fun getEIP4844GasFees(): EIP4844GasFees {
    return getRecentFees().run {
      EIP4844GasFees(
        eip1559GasFees = EIP1559GasFees(
          maxPriorityFeePerGas = min(
            this.eip1559GasFees.maxPriorityFeePerGas,
            config.maxFeePerGasCap
          ),
          maxFeePerGas = min(
            this.eip1559GasFees.maxFeePerGas,
            config.maxFeePerGasCap
          )
        ),
        maxFeePerBlobGas = min(this.maxFeePerBlobGas, config.maxFeePerBlobGasCap)
      )
    }
  }
}
