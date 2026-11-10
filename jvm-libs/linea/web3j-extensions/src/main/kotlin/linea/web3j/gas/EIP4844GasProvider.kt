package linea.web3j.gas

import org.web3j.tx.gas.ContractEIP1559GasProvider
import tech.pegasys.teku.infrastructure.async.SafeFuture

data class EIP4844GasFees(
  val eip1559GasFees: EIP1559GasFees,
  val maxFeePerBlobGas: ULong,
)

interface EIP4844GasProvider : ContractEIP1559GasProvider {
  /** May block; use [getEIP4844GasFeesAsync] from async code. */
  fun getEIP4844GasFees(): EIP4844GasFees

  fun getEIP4844GasFeesAsync(): SafeFuture<EIP4844GasFees>
}
