package linea.contract.l2

import net.consensys.linea.contract.L2MessageService.FUNC_ANCHORL1L2MESSAGEHASHES
import org.web3j.abi.TypeReference
import org.web3j.abi.datatypes.DynamicArray
import org.web3j.abi.datatypes.Function
import org.web3j.abi.datatypes.Type
import org.web3j.abi.datatypes.generated.Bytes32
import org.web3j.abi.datatypes.generated.Uint256
import java.math.BigInteger

internal fun buildAnchorL1L2MessageHashesV1(
  messageHashes: List<ByteArray>,
  startingMessageNumber: BigInteger,
  finalMessageNumber: BigInteger,
  finalRollingHash: ByteArray
): Function {
  return Function(
    /* name = */ FUNC_ANCHORL1L2MESSAGEHASHES,
    /* inputParameters = */ listOf<Type<*>>(
      DynamicArray(
        Bytes32::class.java,
        messageHashes.map { Bytes32(it) }
      ),
      Uint256(startingMessageNumber),
      Uint256(finalMessageNumber),
      Bytes32(finalRollingHash)
    ),
    /* outputParameters = */ emptyList<TypeReference<*>>()
  )
}
