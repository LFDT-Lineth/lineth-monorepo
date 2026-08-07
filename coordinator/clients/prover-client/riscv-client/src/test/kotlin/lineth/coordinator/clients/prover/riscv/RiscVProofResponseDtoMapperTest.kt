package lineth.coordinator.clients.prover.riscv

import lineth.clients.L2ExecutionProofPublicInputs
import lineth.clients.L2ExecutionProofResponseV1
import lineth.clients.RollupAggregationProofResponseV1
import lineth.clients.RollupProofPublicInputs
import lineth.clients.RollupProofResponseV1
import lineth.coordinator.clients.prover.serialization.JsonSerialization
import lineth.kotlin.decodeHex
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Instant

/**
 * Verifies that the RISC-V proof-response DTO -> domain mappers decode every field (DTO `String` (hex) -> domain
 * `ByteArray`, DTO `Long` -> domain `ULong`), and that a JSON response — as it would arrive from a file written by
 * the prover or from a REST response body — deserializes into the response DTO and maps onto the domain type.
 */
class RiscVProofResponseDtoMapperTest {

  private val jsonMapper = JsonSerialization.proofResponseMapperV1

  private val executionPublicInputsDto = L2ExecutionProofPublicInputsDto(
    parentBlockHash = "0x0a",
    endBlockHash = "0x0b",
    endBlockNumber = 1000503,
    endBlockTimestamp = 1763000123,
    l2L1MessagesHash = "0x01",
    parentL1L2BridgeRollingHash = "0x02",
    parentL1L2BridgeRollingHashMessageNumber = 3,
    endL1L2BridgeRollingHash = "0x04",
    endL1L2BridgeRollingHashMessageNumber = 5,
    dynamicChainConfigHash = "0xc0ffee",
    parentFtxRollingHash = "0x06",
    parentProcessedFtxNumber = 7,
    endFtxRollingHash = "0x07",
    endProcessedFtxNumber = 8,
    filteredAddressesHash = "0x09",
    txFromsHash = "0x0c",
  )

  private val expectedExecutionPublicInputs = L2ExecutionProofPublicInputs(
    parentBlockHash = "0x0a".decodeHex(),
    endBlockHash = "0x0b".decodeHex(),
    endBlockNumber = 1000503UL,
    endBlockTimestamp = Instant.fromEpochSeconds(1763000123L),
    l2L1MessagesHash = "0x01".decodeHex(),
    parentL1L2BridgeRollingHash = "0x02".decodeHex(),
    parentL1L2BridgeRollingHashMessageNumber = 3UL,
    endL1L2BridgeRollingHash = "0x04".decodeHex(),
    endL1L2BridgeRollingHashMessageNumber = 5UL,
    dynamicChainConfigHash = "0xc0ffee".decodeHex(),
    parentFtxRollingHash = "0x06".decodeHex(),
    parentFtxNumber = 7UL,
    endFtxRollingHash = "0x07".decodeHex(),
    endFtxNumber = 8UL,
    filteredAddressesHash = "0x09".decodeHex(),
    txFromsHash = "0x0c".decodeHex(),
  )

  private val rollupPublicInputsDto = RollupProofPublicInputsDto(
    endBlockNumber = 1000520,
    endBlockTimestamp = 1763000457,
    l2L1BridgeTransactionTree = "0x10",
    parentL1L2BridgeRollingHash = "0x11",
    parentL1L2BridgeRollingHashMessageNumber = 12,
    endL1L2BridgeRollingHash = "0x13",
    endL1L2BridgeRollingHashMessageNumber = 14,
    dynamicChainConfigHash = "0xc0ffee",
    parentFtxRollingHash = "0x15",
    parentProcessedFtxNumber = 16,
    endFtxRollingHash = "0x16",
    endProcessedFtxNumber = 17,
    filteredAddressesHash = "0x18",
    parentShnarf = "0x19",
    endShnarf = "0x1a",
  )

  private val expectedRollupPublicInputs = RollupProofPublicInputs(
    endBlockNumber = 1000520UL,
    endBlockTimestamp = Instant.fromEpochSeconds(1763000457L),
    l2L1BridgeTransactionTree = "0x10".decodeHex(),
    parentL1L2BridgeMessageRollingHash = "0x11".decodeHex(),
    parentL1L2BridgeMessageNumber = 12UL,
    endL1L2BridgeMessageRollingHash = "0x13".decodeHex(),
    endL1L2BridgeMessageNumber = 14UL,
    dynamicChainConfigHash = "0xc0ffee".decodeHex(),
    parentFtxRollingHash = "0x15".decodeHex(),
    parentFtxNumber = 16UL,
    endFtxRollingHash = "0x16".decodeHex(),
    endFtxNumber = 17UL,
    filteredAddressesHash = "0x18".decodeHex(),
    parentShnarf = "0x19".decodeHex(),
    endShnarf = "0x1a".decodeHex(),
  )

  @Test
  fun `L2ExecutionProofResponseDtoMapper decodes every field`() {
    val dto = L2ExecutionProofResponseDto(
      proverVersion = "4.0.0-riscv",
      startBlockNumber = 1000500L,
      proof = "0xabcd",
      publicInputs = executionPublicInputsDto,
      l2L1Messages = listOf("0xaa"),
      txFroms = listOf("0xbb"),
      filteredAddresses = listOf("0xcc"),
    )

    assertThat(
      L2ExecutionProofResponseDtoMapper(dto),
    ).isEqualTo(
      L2ExecutionProofResponseV1(
        startBlockNumber = 1000500UL,
        endBlockNumber = 1000503UL,
        proof = "0xabcd".decodeHex(),
        publicInputs = expectedExecutionPublicInputs,
        l2L1Messages = listOf("0xaa".decodeHex()),
        txFroms = listOf("0xbb".decodeHex()),
        filteredAddresses = listOf("0xcc".decodeHex()),
      ),
    )
  }

  @Test
  fun `RollupProofResponseDtoMapper decodes every field`() {
    val dto = RollupProofResponseDto(
      proverVersion = "4.0.0-riscv",
      startBlockNumber = 1000500L,
      proof = "0xabcd",
      publicInputs = rollupPublicInputsDto,
      l2L1Roots = listOf("0xaa"),
      filteredAddresses = listOf("0xbb"),
    )

    assertThat(
      RollupProofResponseDtoMapper(dto),
    ).isEqualTo(
      RollupProofResponseV1(
        startBlockNumber = 1000500UL,
        endBlockNumber = 1000520UL,
        proof = "0xabcd".decodeHex(),
        publicInputs = expectedRollupPublicInputs,
        l2L1Roots = listOf("0xaa".decodeHex()),
        filteredAddresses = listOf("0xbb".decodeHex()),
      ),
    )
  }

  @Test
  fun `RollupAggregationProofResponseDtoMapper decodes every field`() {
    val dto = RollupAggregationProofResponseDto(
      proverVersion = "4.0.0-riscv",
      proof = "0xabcd",
      startBlockNumber = 1000500L,
      publicInputs = rollupPublicInputsDto,
      l2L1Roots = listOf("0xaa", "0xcc"),
      filteredAddresses = listOf("0xbb", "0xdd"),
      l2MessagingBlocksOffsets = listOf(1, 20, 100),
    )

    assertThat(
      RollupAggregationProofResponseDtoMapper(dto),
    ).isEqualTo(
      RollupAggregationProofResponseV1(
        startBlockNumber = 1000500UL,
        endBlockNumber = 1000520UL,
        proof = "0xabcd".decodeHex(),
        publicInputs = expectedRollupPublicInputs,
        l2L1Roots = listOf("0xaa".decodeHex(), "0xcc".decodeHex()),
        filteredAddresses = listOf("0xbb".decodeHex(), "0xdd".decodeHex()),
        l2MessagingBlocksOffsets = listOf(1UL, 20UL, 100UL),
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
          "l2L1MessagesHash": "0x01",
          "parentL1L2BridgeRollingHash": "0x02",
          "parentL1L2BridgeRollingHashMessageNumber": 3,
          "endL1L2BridgeRollingHash": "0x04",
          "endL1L2BridgeRollingHashMessageNumber": 5,
          "dynamicChainConfigHash": "0xc0ffee",
          "parentFtxRollingHash": "0x06",
          "parentProcessedFtxNumber": 7,
          "endFtxRollingHash": "0x07",
          "endProcessedFtxNumber": 8,
          "filteredAddressesHash": "0x09",
          "txFromsHash": "0x0c"
        },
        "l2L1Messages": ["0xaa"],
        "txFroms": ["0xbb"],
        "filteredAddresses": []
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
        l2L1Messages = listOf("0xaa".decodeHex()),
        txFroms = listOf("0xbb".decodeHex()),
        filteredAddresses = emptyList(),
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
          "l2L1BridgeTransactionTree": "0x10",
          "parentL1L2BridgeRollingHash": "0x11",
          "parentL1L2BridgeRollingHashMessageNumber": 12,
          "endL1L2BridgeRollingHash": "0x13",
          "endL1L2BridgeRollingHashMessageNumber": 14,
          "dynamicChainConfigHash": "0xc0ffee",
          "parentFtxRollingHash": "0x15",
          "parentProcessedFtxNumber": 16,
          "endFtxRollingHash": "0x16",
          "endProcessedFtxNumber": 17,
          "filteredAddressesHash": "0x18",
          "parentShnarf": "0x19",
          "endShnarf": "0x1a"
        },
        "l2L1Roots": ["0xaa"],
        "filteredAddresses": []
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
        l2L1Roots = listOf("0xaa".decodeHex()),
        filteredAddresses = emptyList(),
      ),
    )
  }
}
