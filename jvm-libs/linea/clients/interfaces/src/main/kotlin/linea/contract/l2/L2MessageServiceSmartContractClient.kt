package linea.contract.l2

import linea.contract.l1.ContractVersionProvider
import linea.domain.BlockParameter
import tech.pegasys.teku.infrastructure.async.SafeFuture

enum class L2MessageServiceSmartContractVersion : Comparable<L2MessageServiceSmartContractVersion> {
  V1, // initial version
}

interface L2MessageServiceSmartContractClientReadOnly : ContractVersionProvider<L2MessageServiceSmartContractVersion> {
  fun getAddress(): String
  fun getDeploymentBlock(): SafeFuture<ULong>
  fun getLastAnchoredL1MessageNumber(block: BlockParameter): SafeFuture<ULong>
  fun getRollingHashByL1MessageNumber(
    block: BlockParameter,
    l1MessageNumber: ULong,
  ): SafeFuture<ByteArray>
}

interface L2MessageServiceSmartContractClient : L2MessageServiceSmartContractClientReadOnly {
  /**
   * @notice Add cross-chain L1->L2 message hashes in storage.
   * @dev Only address that has the role 'L1_L2_MESSAGE_SETTER_ROLE' are allowed to call this function.
   * @dev NB: In the unlikely event of a duplicate anchoring, the lastAnchoredL1MessageNumber MUST NOT be incremented.
   * @dev and the rolling hash not calculated, else synchronisation will break.
   * @dev If starting number is zero, an underflow error is expected.
   * @param _messageHashes New message hashes to anchor on L2.
   * @param _startingMessageNumber The expected L1 message number to start when anchoring.
   * @param _finalMessageNumber The expected L1 message number to end on when anchoring.
   * @param _finalRollingHash The expected L1 rolling hash to end on when anchoring.
   * @return The transaction hash of the anchoring.
   * <pre>
   * <code>
   * function anchorL1L2MessageHashes(
   *   bytes32[] calldata _messageHashes,
   *   uint256 _startingMessageNumber,
   *   uint256 _finalMessageNumber,
   *   bytes32 _finalRollingHash
   * )
   * </code>
   * </pre>
   */
  fun anchorL1L2MessageHashes(
    messageHashes: List<ByteArray>,
    startingMessageNumber: ULong,
    finalMessageNumber: ULong,
    finalRollingHash: ByteArray,
  ): SafeFuture<String>
}
