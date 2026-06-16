package linea.coordinator.clients.prover.riscv

import linea.clients.ChainConfig
import linea.clients.ExecutionPayload
import linea.clients.ExecutionRequests
import linea.clients.ExecutionWitness
import linea.clients.Withdrawal
import org.apache.tuweni.bytes.Bytes
import org.apache.tuweni.units.bigints.UInt256
import tech.pegasys.teku.infrastructure.ssz.SszData
import tech.pegasys.teku.infrastructure.ssz.impl.SszContainerImpl
import tech.pegasys.teku.infrastructure.ssz.primitive.SszUInt256
import tech.pegasys.teku.infrastructure.ssz.primitive.SszUInt64
import tech.pegasys.teku.infrastructure.unsigned.UInt64
import java.math.BigInteger

/**
 * [StatelessInputSszSerializer] backed by teku SSZ, serializing from the domain types.
 *
 * Uses the shared [StatelessInputSszSchemas] (mirroring the Python `SszStatelessInput` schema) so it produces the same
 * bytes as [L2ExecutionStatelessInputDtoSszSerializer] for equivalent inputs.
 *
 * Current simplifying assumptions (see method docs): `versionedHashes`, `parentBeaconBlockRoot`, `executionRequests`
 * and `publicKey` are treated as empty/zero.
 */
class L2ExecutionStatelessInputSszSerializer : StatelessInputSszSerializer {

  override fun getStatelessInputSsz(
    executionPayload: ExecutionPayload,
    executionWitness: ExecutionWitness,
    chainConfig: ChainConfig,
    versionedHashes: List<ByteArray>,
    parentBeaconBlockRoot: ByteArray,
    executionRequests: ExecutionRequests,
    publicKey: List<ByteArray>,
  ): ByteArray {
    val statelessInput = StatelessInputSszSchemas.statelessInput.createFromFieldValues(
      listOf<SszData>(
        newPayloadRequestValue(executionPayload, versionedHashes, parentBeaconBlockRoot),
        executionWitnessValue(executionWitness),
        chainConfigValue(chainConfig),
        // TODO: public keys are assumed empty for now.
        StatelessInputSszSchemas.publicKeys.createFromElements(
          publicKey.map { StatelessInputSszSchemas.publicKeyBytes.fromBytes(Bytes.wrap(it)) },
        ),
      ),
    )
    return statelessInput.sszSerialize().toArray()
  }

  private fun newPayloadRequestValue(
    payload: ExecutionPayload,
    versionedHashes: List<ByteArray>,
    parentBeaconBlockRoot: ByteArray,
  ): SszData {
    return StatelessInputSszSchemas.newPayloadRequest.createFromFieldValues(
      listOf<SszData>(
        executionPayloadValue(payload),
        // TODO: versioned hashes are assumed empty for now.
        StatelessInputSszSchemas.versionedHashes.createFromElements(
          versionedHashes.map { StatelessInputSszSchemas.bytes32.fromBytes(Bytes.wrap(it)) },
        ),
        // TODO: parent beacon block root is assumed empty (zero) for now.
        StatelessInputSszSchemas.bytes32.fromBytes(Bytes.wrap(bytes32OrZero(parentBeaconBlockRoot))),
        // TODO: execution requests are assumed empty for now.
        StatelessInputSszSchemas.executionRequests.createFromElements(emptyList()),
      ),
    )
  }

  private fun executionPayloadValue(payload: ExecutionPayload): SszData {
    return StatelessInputSszSchemas.executionPayload.createFromFieldValues(
      listOf<SszData>(
        StatelessInputSszSchemas.bytes32.fromBytes(Bytes.wrap(payload.parentHash)),
        StatelessInputSszSchemas.address.fromBytes(Bytes.wrap(payload.feeRecipient)),
        StatelessInputSszSchemas.bytes32.fromBytes(Bytes.wrap(payload.stateRoot)),
        StatelessInputSszSchemas.bytes32.fromBytes(Bytes.wrap(payload.receiptsRoot)),
        StatelessInputSszSchemas.bloom.fromBytes(Bytes.wrap(payload.logsBloom)),
        StatelessInputSszSchemas.bytes32.fromBytes(Bytes.wrap(payload.prevRandao)),
        uint64(payload.blockNumber),
        uint64(payload.gasLimit),
        uint64(payload.gasUsed),
        uint64(payload.timestamp),
        StatelessInputSszSchemas.extraData.fromBytes(Bytes.wrap(payload.extraData)),
        SszUInt256.of(UInt256.valueOf(payload.baseFeePerGas)),
        StatelessInputSszSchemas.bytes32.fromBytes(Bytes.wrap(payload.blockHash)),
        StatelessInputSszSchemas.transactions.createFromElements(
          payload.transactions.map { StatelessInputSszSchemas.transactionBytes.fromBytes(Bytes.wrap(it)) },
        ),
        StatelessInputSszSchemas.withdrawals.createFromElements(payload.withdrawals.map { withdrawalValue(it) }),
        uint64(payload.blobGasUsed),
        uint64(payload.excessBlobGas),
        StatelessInputSszSchemas.blockAccessList.fromBytes(Bytes.wrap(payload.blockAccessList)),
      ),
    )
  }

  private fun withdrawalValue(withdrawal: Withdrawal): SszContainerImpl {
    return StatelessInputSszSchemas.withdrawal.createFromFieldValues(
      listOf<SszData>(
        uint64(withdrawal.index),
        uint64(withdrawal.validatorIndex),
        StatelessInputSszSchemas.address.fromBytes(Bytes.wrap(withdrawal.address)),
        SszUInt256.of(UInt256.valueOf(BigInteger(withdrawal.amount.toString()))),
      ),
    )
  }

  private fun executionWitnessValue(witness: ExecutionWitness): SszData {
    return StatelessInputSszSchemas.executionWitness.createFromFieldValues(
      listOf<SszData>(
        StatelessInputSszSchemas.witnessState.createFromElements(
          witness.state.map { StatelessInputSszSchemas.witnessNodeBytes.fromBytes(Bytes.wrap(it)) },
        ),
        StatelessInputSszSchemas.witnessCodes.createFromElements(
          witness.codes.map { StatelessInputSszSchemas.witnessCodeBytes.fromBytes(Bytes.wrap(it)) },
        ),
        StatelessInputSszSchemas.witnessHeaders.createFromElements(
          witness.headers.map { StatelessInputSszSchemas.witnessHeaderBytes.fromBytes(Bytes.wrap(it)) },
        ),
      ),
    )
  }

  private fun chainConfigValue(chainConfig: ChainConfig): SszData {
    return StatelessInputSszSchemas.chainConfig.createFromFieldValues(listOf<SszData>(uint64(chainConfig.chainId)))
  }

  private fun uint64(value: ULong): SszUInt64 = SszUInt64.of(UInt64.fromLongBits(value.toLong()))

  private fun bytes32OrZero(bytes: ByteArray): ByteArray =
    if (bytes.size == BYTES32_LENGTH) bytes else ByteArray(BYTES32_LENGTH)

  private companion object {
    private const val BYTES32_LENGTH = 32
  }
}
