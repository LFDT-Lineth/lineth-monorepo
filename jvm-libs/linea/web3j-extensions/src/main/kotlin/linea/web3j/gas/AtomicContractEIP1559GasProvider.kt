package linea.web3j.gas

import org.web3j.tx.gas.ContractEIP1559GasProvider
import tech.pegasys.teku.infrastructure.async.SafeFuture

data class EIP1559GasFees(
  val maxPriorityFeePerGas: ULong,
  val maxFeePerGas: ULong,
)

interface AtomicContractEIP1559GasProvider : ContractEIP1559GasProvider {
  fun getEIP1559GasFeesAsync(): SafeFuture<EIP1559GasFees>

  /** Blocks until the fees are resolved; use [getEIP1559GasFeesAsync] from async code. */
  fun getEIP1559GasFees(): EIP1559GasFees = getEIP1559GasFeesAsync().get()
}
