package linea.ethapi

import linea.domain.BlockParameter
import linea.domain.TransactionForEthCall
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger

data class StateOverride(
  val balance: BigInteger? = null, // Temporary account balance for the call execution.
  val nonce: ULong? = null, // Temporary nonce value for the call execution.
  val code: ByteArray? = null, // Bytecode to inject into the account.
  // Data, 20 bytes	Address to which the precompile address should be moved.
  val movePrecompileToAddress: ByteArray? = null,
  // key:value pairs (ByteArray hexadecimal encoded) to override all slots in the account storage.
  // You cannot set both the state and stateDiff options simultaneously.
  val state: Map<String, String>? = null,
  // key:value pairs (ByteArray hexadecimal encoded) to override individual slots in the account storage.
  // You cannot set both the state and stateDiff options simultaneously.
  val stateDiff: Map<String, String>? = null,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as StateOverride

    if (balance != other.balance) return false
    if (nonce != other.nonce) return false
    if (!code.contentEquals(other.code)) return false
    if (!movePrecompileToAddress.contentEquals(other.movePrecompileToAddress)) return false
    if (state != other.state) return false
    if (stateDiff != other.stateDiff) return false

    return true
  }

  override fun hashCode(): Int {
    var result = balance.hashCode()
    result = 31 * result + nonce.hashCode()
    result = 31 * result + code.contentHashCode()
    result = 31 * result + movePrecompileToAddress.contentHashCode()
    result = 31 * result + state.hashCode()
    result = 31 * result + stateDiff.hashCode()
    return result
  }
}

interface EthApiSimulationClient {
  fun ethCall(
    transaction: TransactionForEthCall,
    blockParameter: BlockParameter = BlockParameter.Tag.LATEST,
    stateOverride: StateOverride? = null,
  ): SafeFuture<ByteArray>

  fun ethEstimateGas(transaction: TransactionForEthCall): SafeFuture<ULong>
}
