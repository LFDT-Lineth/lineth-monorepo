package linea.web3j.gas

import org.web3j.tx.gas.ContractEIP1559GasProvider

data class EIP4844GasFees(
  val eip1559GasFees: EIP1559GasFees,
  val maxFeePerBlobGas: ULong
)

interface EIP4844GasProvider : ContractEIP1559GasProvider {
  fun getEIP4844GasFees(): EIP4844GasFees
}
