package lineth.coordinator.clients.prover

import linea.clients.FinalizationPublicInputs
import linea.clients.L2ExecutionProofPublicInputs
import linea.clients.L2ExecutionProofResponseV1
import linea.clients.RollupAggregationProofResponseV1
import linea.clients.RollupProofPublicInputs
import linea.clients.RollupProofResponseV1
import linea.kotlin.decodeHex
import lineth.coordinator.clients.prover.serialization.JsonSerialization
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Instant

/**
 * Verifies that the RISC-V proof-response DTO -> domain mappers decode every field (DTO `String` (hex) -> domain
 * `ByteArray`, DTO `Long` -> domain `ULong`), and that a JSON response — as it would arrive from a file written by
 * the prover or from a REST response body — deserializes into the response DTO and maps onto the domain type.
 */
class RiscvProofResponseDtoMapperTest {

  private val jsonMapper = JsonSerialization.proofResponseMapperV1

  private val executionPublicInputsDto = L2ExecutionProofPublicInputsDto(
    parentBlockHash = "0x0a",
    endBlockHash = "0x0b",
    endBlockNumber = 1000503,
    endBlockTimestamp = 1763000123,
    l2L1Messages = listOf("0xaa"),
    endL1L2BridgeRollingHash = "0x04",
    endL1L2BridgeRollingHashMessageNumber = 5,
    dynamicChainConfigHash = "0xc0ffee",
    parentFtxRollingHash = "0x06",
    parentFtxNumber = 7,
    endFtxRollingHash = "0x07",
    endProcessedFtxNumber = 8,
    filteredAddresses = listOf("0x09"),
    txFromsHash = "0x0c",
    blockCount = 4,
    l2MessagingBlocksOffsets = listOf(1, 3),
  )

  private val expectedExecutionPublicInputs = L2ExecutionProofPublicInputs(
    parentBlockHash = "0x0a".decodeHex(),
    endBlockHash = "0x0b".decodeHex(),
    endBlockNumber = 1000503UL,
    endBlockTimestamp = Instant.fromEpochSeconds(1763000123L),
    l2L1Messages = listOf("0xaa".decodeHex()),
    endL1L2BridgeRollingHash = "0x04".decodeHex(),
    endL1L2BridgeRollingHashMessageNumber = 5UL,
    dynamicChainConfigHash = "0xc0ffee".decodeHex(),
    parentFtxRollingHash = "0x06".decodeHex(),
    parentFtxNumber = 7UL,
    endFtxRollingHash = "0x07".decodeHex(),
    endFtxNumber = 8UL,
    filteredAddresses = listOf("0x09".decodeHex()),
    txFromsHash = "0x0c".decodeHex(),
    blockCount = 4UL,
    l2MessagingBlocksOffsets = listOf(1UL, 3UL),
  )

  private val rollupPublicInputsDto = RollupProofPublicInputsDto(
    endBlockNumber = 1000520,
    endBlockTimestamp = 1763000457,
    endL1L2BridgeRollingHash = "0x13",
    endL1L2BridgeRollingHashMessageNumber = 14,
    dynamicChainConfigHash = "0xc0ffee",
    parentFtxRollingHash = "0x15",
    parentFtxNumber = 16,
    endFtxRollingHash = "0x16",
    endProcessedFtxNumber = 17,
    parentDataRollingHash = "0x19",
    endDataRollingHash = "0x1a",
    parentBlockHash = "0x1b",
    endBlockHash = "0x1c",
    parentDataTailTakeBytes = 0,
    finalDataTailDiscardBytes = 1000,
    l2L1Messages = listOf("0xaa"),
    filteredAddresses = listOf("0xbb"),
    programVks = listOf("0xdd"),
    blockCount = 20,
    l2MessagingBlocksOffsets = listOf(2, 7),
  )

  private val expectedRollupPublicInputs = RollupProofPublicInputs(
    endBlockNumber = 1000520UL,
    endBlockTimestamp = Instant.fromEpochSeconds(1763000457L),
    endL1L2BridgeMessageRollingHash = "0x13".decodeHex(),
    endL1L2BridgeMessageNumber = 14UL,
    dynamicChainConfigHash = "0xc0ffee".decodeHex(),
    parentFtxRollingHash = "0x15".decodeHex(),
    parentFtxNumber = 16UL,
    endFtxRollingHash = "0x16".decodeHex(),
    endFtxNumber = 17UL,
    parentDataRollingHash = "0x19".decodeHex(),
    endDataRollingHash = "0x1a".decodeHex(),
    parentBlockHash = "0x1b".decodeHex(),
    endBlockHash = "0x1c".decodeHex(),
    parentDataTailTakeBytes = 0,
    finalDataTailDiscardBytes = 1000,
    l2L1Messages = listOf("0xaa".decodeHex()),
    filteredAddresses = listOf("0xbb".decodeHex()),
    programVks = listOf("0xdd".decodeHex()),
    blockCount = 20UL,
    l2MessagingBlocksOffsets = listOf(2UL, 7UL),
  )

  @Test
  fun `L2ExecutionProofResponseDtoMapper decodes every field`() {
    val programVkHex = "0x" + "dd".repeat(32)
    val dto = L2ExecutionProofResponseDto(
      proverVersion = "4.0.0-riscv",
      startBlockNumber = 1000500L,
      proof = "0xabcd",
      publicInputs = executionPublicInputsDto,
      programVk = programVkHex,
    )

    assertThat(
      L2ExecutionProofResponseDtoMapper(dto),
    ).isEqualTo(
      L2ExecutionProofResponseV1(
        startBlockNumber = 1000500UL,
        endBlockNumber = 1000503UL,
        proof = "0xabcd".decodeHex(),
        publicInputs = expectedExecutionPublicInputs,
        programVk = programVkHex.decodeHex(),
      ),
    )
  }

  @Test
  fun `RollupProofResponseDtoMapper decodes every field`() {
    val programVkHex = "0x" + "bb".repeat(32)
    val dto = RollupProofResponseDto(
      proverVersion = "4.0.0-riscv",
      programVk = programVkHex,
      proof = "0xabcd",
      startBlockNumber = 1000500L,
      publicInputs = rollupPublicInputsDto,
    )

    assertThat(
      RollupProofResponseDtoMapper(dto),
    ).isEqualTo(
      RollupProofResponseV1(
        startBlockNumber = 1000500UL,
        endBlockNumber = 1000520UL,
        proof = "0xabcd".decodeHex(),
        publicInputs = expectedRollupPublicInputs,
        programVk = programVkHex.decodeHex(),
      ),
    )
  }

  @Test
  fun `RollupAggregationProofResponseDtoMapper decodes every field`() {
    val dto = RollupAggregationProofResponseDto(
      proverVersion = "4.0.0-riscv",
      proof = "0xabcd",
      startBlockNumber = 1000500L,
      publicInputs = FinalizationPublicInputsDto(
        endBlockNumber = 1000520,
        endBlockTimestamp = 1763000457,
        endL1L2BridgeRollingHash = "0x13",
        endL1L2BridgeRollingHashMessageNumber = 14,
        dynamicChainConfigHash = "0xc0ffee",
        parentFtxRollingHash = "0x15",
        parentFtxNumber = 16,
        endFtxRollingHash = "0x16",
        endProcessedFtxNumber = 17,
        parentDataRollingHash = "0x19",
        endDataRollingHash = "0x1a",
        parentBlockHash = "0x1b",
        endBlockHash = "0x1c",
        parentDataTailTakeBytes = 3,
        finalDataTailDiscardBytes = 1000,
        l2L1Roots = listOf("0xaa", "0xcc"),
        l2L1TreeDepth = 5,
        filteredAddresses = listOf("0xbb", "0xdd"),
        programIds = listOf("0xee"),
        l2MessagingBlocksOffsets = listOf(1, 20, 100),
      ),
    )

    assertThat(
      RollupAggregationProofResponseDtoMapper(dto),
    ).isEqualTo(
      RollupAggregationProofResponseV1(
        startBlockNumber = 1000500UL,
        endBlockNumber = 1000520UL,
        proof = "0xabcd".decodeHex(),
        publicInputs = FinalizationPublicInputs(
          endBlockNumber = 1000520UL,
          endBlockTimestamp = Instant.fromEpochSeconds(1763000457L),
          endL1L2BridgeMessageRollingHash = "0x13".decodeHex(),
          endL1L2BridgeMessageNumber = 14UL,
          dynamicChainConfigHash = "0xc0ffee".decodeHex(),
          parentFtxRollingHash = "0x15".decodeHex(),
          parentFtxNumber = 16UL,
          endFtxRollingHash = "0x16".decodeHex(),
          endFtxNumber = 17UL,
          parentDataRollingHash = "0x19".decodeHex(),
          endDataRollingHash = "0x1a".decodeHex(),
          parentBlockHash = "0x1b".decodeHex(),
          endBlockHash = "0x1c".decodeHex(),
          parentDataTailTakeBytes = 3,
          finalDataTailDiscardBytes = 1000,
          l2L1Roots = listOf("0xaa".decodeHex(), "0xcc".decodeHex()),
          l2L1TreeDepth = 5UL,
          filteredAddresses = listOf("0xbb".decodeHex(), "0xdd".decodeHex()),
          programIds = listOf("0xee".decodeHex()),
          l2MessagingBlocksOffsets = listOf(1UL, 20UL, 100UL),
        ),
      ),
    )
  }

  @Test
  fun `L2 execution proof response JSON parses into the DTO and maps to the domain response`() {
    val json = """
      {
        "proverVersion": "4.0.0-riscv",
        "proof": "0xabcd",
        "startBlockNumber": 1000500,
        "publicInputs": {
          "parentBlockHash": "0x0a",
          "endBlockHash": "0x0b",
          "endBlockNumber": 1000503,
          "endBlockTimestamp": 1763000123,
          "l2L1Messages": ["0xaa"],
          "endL1L2BridgeRollingHash": "0x04",
          "endL1L2BridgeRollingHashMessageNumber": 5,
          "dynamicChainConfigHash": "0xc0ffee",
          "parentFtxRollingHash": "0x06",
          "parentFtxNumber": 7,
          "endFtxRollingHash": "0x07",
          "endProcessedFtxNumber": 8,
          "filteredAddresses": ["0x09"],
          "txFromsHash": "0x0c",
          "blockCount": 4,
          "l2MessagingBlocksOffsets": [1, 3]
        },
        "programVk": "0xdddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
      }
    """.trimIndent()

    val dto = jsonMapper.readValue(json, L2ExecutionProofResponseDto::class.java)

    assertThat(
      L2ExecutionProofResponseDtoMapper(dto),
    ).isEqualTo(
      L2ExecutionProofResponseV1(
        startBlockNumber = 1000500UL,
        endBlockNumber = 1000503UL,
        proof = "0xabcd".decodeHex(),
        publicInputs = expectedExecutionPublicInputs,
        programVk = ByteArray(32) { 0xdd.toByte() },
      ),
    )
  }

  @Test
  fun `rollup proof response JSON parses into the DTO and maps to the domain response`() {
    val json = """
      {
        "proverVersion": "4.0.0-riscv",
        "proof": "0xabcd",
        "startBlockNumber": 1000500,
        "publicInputs": {
          "endBlockNumber": 1000520,
          "endBlockTimestamp": 1763000457,
          "endL1L2BridgeRollingHash": "0x13",
          "endL1L2BridgeRollingHashMessageNumber": 14,
          "dynamicChainConfigHash": "0xc0ffee",
          "parentFtxRollingHash": "0x15",
          "parentFtxNumber": 16,
          "endFtxRollingHash": "0x16",
          "endProcessedFtxNumber": 17,
          "parentDataRollingHash": "0x19",
          "endDataRollingHash": "0x1a",
          "parentBlockHash": "0x1b",
          "endBlockHash": "0x1c",
          "parentDataTailTakeBytes": 0,
          "finalDataTailDiscardBytes": 1000,
          "l2L1Messages": ["0xaa"],
          "filteredAddresses": ["0xbb"],
          "programVks": ["0xdd"],
          "blockCount": 20,
          "l2MessagingBlocksOffsets": [2, 7]
        },
        "programVk": "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
      }
    """.trimIndent()

    val dto = jsonMapper.readValue(json, RollupProofResponseDto::class.java)
    val mappedDto = RollupProofResponseDtoMapper(dto)

    assertThat(mappedDto).isEqualTo(
      RollupProofResponseV1(
        startBlockNumber = 1000500UL,
        endBlockNumber = 1000520UL,
        proof = "0xabcd".decodeHex(),
        publicInputs = expectedRollupPublicInputs,
        programVk = ByteArray(32) { 0xbb.toByte() },
      ),
    )
  }
}
