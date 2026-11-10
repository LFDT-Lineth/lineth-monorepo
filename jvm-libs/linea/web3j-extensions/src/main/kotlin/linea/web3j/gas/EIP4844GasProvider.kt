package linea.web3j.gas

import org.web3j.tx.gas.ContractEIP1559GasProvider
import tech.pegasys.teku.infrastructure.async.SafeFuture

data class EIP4844GasFees(
  val eip1559GasFees: EIP1559GasFees,
  val maxFeePerBlobGas: ULong,
)

interface EIP4844GasProvider : ContractEIP1559GasProvider {
  /**
   * Synchronous variant, kept for blocking callers only.
   * Implementations may block the calling thread while fees are fetched,
   * so it must not be called from a Vert.x event-loop thread or async continuations.
   * Prefer [getEIP4844GasFeesAsync].
   */
  fun getEIP4844GasFees(): EIP4844GasFees

  /**
   * Non-blocking variant. Implementations must not block the calling thread.
   */
  fun getEIP4844GasFeesAsync(): SafeFuture<EIP4844GasFees>
}
