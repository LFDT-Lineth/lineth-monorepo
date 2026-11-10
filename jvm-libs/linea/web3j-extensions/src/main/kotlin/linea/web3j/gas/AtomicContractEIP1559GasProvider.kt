package linea.web3j.gas

import org.web3j.tx.gas.ContractEIP1559GasProvider
import tech.pegasys.teku.infrastructure.async.SafeFuture

data class EIP1559GasFees(
  val maxPriorityFeePerGas: ULong,
  val maxFeePerGas: ULong,
)

interface AtomicContractEIP1559GasProvider : ContractEIP1559GasProvider {
  /**
   * Synchronous variant, kept for blocking callers only.
   * Implementations may block the calling thread while fees are fetched,
   * so it must not be called from a Vert.x event-loop thread or async continuations.
   * Prefer [getEIP1559GasFeesAsync].
   */
  fun getEIP1559GasFees(): EIP1559GasFees

  /**
   * Non-blocking variant. Implementations must not block the calling thread.
   */
  fun getEIP1559GasFeesAsync(): SafeFuture<EIP1559GasFees>
}
