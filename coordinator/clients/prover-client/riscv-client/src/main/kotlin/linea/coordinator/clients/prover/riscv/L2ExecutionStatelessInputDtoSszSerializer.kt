package linea.coordinator.clients.prover.riscv

import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import org.apache.tuweni.bytes.Bytes
import org.apache.tuweni.units.bigints.UInt256
import tech.pegasys.teku.infrastructure.ssz.SszData
import tech.pegasys.teku.infrastructure.ssz.impl.SszContainerImpl
import tech.pegasys.teku.infrastructure.ssz.primitive.SszUInt256
import tech.pegasys.teku.infrastructure.ssz.primitive.SszUInt64
import tech.pegasys.teku.infrastructure.unsigned.UInt64
import java.math.BigInteger

/**
 * [StatelessInputDtoSszSerializer] backed by teku SSZ, serializing from the [StatelessInputDto].
 *
 * Uses the shared [StatelessInputSszSchemas] (mirroring the Python `SszStatelessInput` schema) so it produces the same
 * bytes as [L2ExecutionStatelessInputSszSerializer] for equivalent inputs. DTO values are hex strings (decoded to bytes)
 * and `Long`s (treated as unsigned for the SSZ `uint64` fields). Per the Python schema, `ExecutionWitnessDto.keys`,
 * `StatelessChainConfigDto.forkName` have no SSZ field and are dropped.
 */
class L2ExecutionStatelessInputDtoSszSerializer : StatelessInputDtoSszSerializer {

  override fun getStatelessInputDtoSsz(statelessInputDto: StatelessInputDto): String {
    val statelessInput = StatelessInputSszSchemas.statelessInput.createFromFieldValues(
      listOf<SszData>(
        newPayloadRequestValue(statelessInputDto.newPayloadRequest),
        executionWitnessValue(statelessInputDto.executionWitness),
        chainConfigValue(statelessInputDto.chainConfig),
        StatelessInputSszSchemas.publicKeys.createFromElements(
          statelessInputDto.publicKeys.map { StatelessInputSszSchemas.publicKeyBytes.fromBytes(bytes(it)) },
        ),
      ),
    )
    return statelessInput.sszSerialize().toArray().encodeHex()
  }

  private fun newPayloadRequestValue(dto: NewPayloadRequestDto): SszData {
    return StatelessInputSszSchemas.newPayloadRequest.createFromFieldValues(
      listOf<SszData>(
        executionPayloadValue(dto.executionPayload),
        StatelessInputSszSchemas.versionedHashes.createFromElements(
          dto.versionedHashes.map { StatelessInputSszSchemas.bytes32.fromBytes(bytes(it)) },
        ),
        StatelessInputSszSchemas.bytes32.fromBytes(bytes(dto.parentBeaconBlockRoot)),
        StatelessInputSszSchemas.executionRequests.createFromElements(
          executionRequestEntries(
            dto.executionRequests,
          ).map { StatelessInputSszSchemas.requestBytes.fromBytes(bytes(it)) },
        ),
      ),
    )
  }

  private fun executionPayloadValue(dto: ExecutionPayloadDto): SszData {
    return StatelessInputSszSchemas.executionPayload.createFromFieldValues(
      listOf<SszData>(
        StatelessInputSszSchemas.bytes32.fromBytes(bytes(dto.parentHash)),
        StatelessInputSszSchemas.address.fromBytes(bytes(dto.feeRecipient)),
        StatelessInputSszSchemas.bytes32.fromBytes(bytes(dto.stateRoot)),
        StatelessInputSszSchemas.bytes32.fromBytes(bytes(dto.receiptsRoot)),
        StatelessInputSszSchemas.bloom.fromBytes(bytes(dto.logsBloom)),
        StatelessInputSszSchemas.bytes32.fromBytes(bytes(dto.prevRandao)),
        uint64(dto.blockNumber),
        uint64(dto.gasLimit),
        uint64(dto.gasUsed),
        uint64(dto.timestamp),
        StatelessInputSszSchemas.extraData.fromBytes(bytes(dto.extraData)),
        SszUInt256.of(UInt256.valueOf(dto.baseFeePerGas)),
        StatelessInputSszSchemas.bytes32.fromBytes(bytes(dto.blockHash)),
        StatelessInputSszSchemas.transactions.createFromElements(
          dto.transactions.map { StatelessInputSszSchemas.transactionBytes.fromBytes(bytes(it)) },
        ),
        StatelessInputSszSchemas.withdrawals.createFromElements(dto.withdrawals.map { withdrawalValue(it) }),
        uint64(dto.blobGasUsed),
        uint64(dto.excessBlobGas),
        StatelessInputSszSchemas.blockAccessList.fromBytes(bytes(dto.blockAccessList)),
      ),
    )
  }

  private fun withdrawalValue(dto: WithdrawalDto): SszContainerImpl {
    return StatelessInputSszSchemas.withdrawal.createFromFieldValues(
      listOf<SszData>(
        uint64(dto.index),
        uint64(dto.validatorIndex),
        StatelessInputSszSchemas.address.fromBytes(bytes(dto.address)),
        SszUInt256.of(UInt256.valueOf(BigInteger.valueOf(dto.amount))),
      ),
    )
  }

  private fun executionWitnessValue(dto: ExecutionWitnessDto): SszData {
    return StatelessInputSszSchemas.executionWitness.createFromFieldValues(
      listOf<SszData>(
        StatelessInputSszSchemas.witnessState.createFromElements(
          dto.state.map { StatelessInputSszSchemas.witnessNodeBytes.fromBytes(bytes(it)) },
        ),
        StatelessInputSszSchemas.witnessCodes.createFromElements(
          dto.codes.map { StatelessInputSszSchemas.witnessCodeBytes.fromBytes(bytes(it)) },
        ),
        StatelessInputSszSchemas.witnessHeaders.createFromElements(
          dto.headers.map { StatelessInputSszSchemas.witnessHeaderBytes.fromBytes(bytes(it)) },
        ),
      ),
    )
  }

  private fun chainConfigValue(dto: StatelessChainConfigDto): SszData {
    return StatelessInputSszSchemas.chainConfig.createFromFieldValues(listOf<SszData>(uint64(dto.chainId)))
  }

  /**
   * Flattens the structured [ExecutionRequestsDto] into the positional list of opaque request byte-lists the SSZ
   * `execution_requests` field expects, in deposits -> withdrawals -> consolidations order.
   */
  private fun executionRequestEntries(dto: ExecutionRequestsDto): List<String> =
    dto.deposits + dto.withdrawals + dto.consolidations

  private fun uint64(value: Long): SszUInt64 = SszUInt64.of(UInt64.fromLongBits(value))

  private fun bytes(hex: String): Bytes = Bytes.wrap(hex.decodeHex())
}
