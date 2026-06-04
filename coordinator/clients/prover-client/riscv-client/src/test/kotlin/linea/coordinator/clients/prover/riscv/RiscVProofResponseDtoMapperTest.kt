package linea.coordinator.clients.prover.riscv

import linea.clients.L2ExecutionProofPublicInputs
import linea.clients.L2ExecutionProofResponse
import linea.clients.RollupAggregationProofResponse
import linea.clients.RollupProofPublicInputs
import linea.clients.RollupProofResponse
import linea.coordinator.clients.prover.serialization.JsonSerialization
import linea.domain.AggregationProofIndex
import linea.domain.CompressionProofIndex
import linea.domain.ExecutionProofIndex
import linea.kotlin.decodeHex
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

  private fun executionPublicInputsDto(): L2ExecutionProofPublicInputsDto = L2ExecutionProofPublicInputsDto(
    parentBlockHash = "0x0a",
    endBlockHash = "0x0b",
    endBlockNumber = 1000503,
    endBlockTimestamp = 1763000123,
    L2L1MessagesHash = "0x01",
    parentL1L2BridgeRollingHash = "0x02",
    parentL1L2BridgeRollingHashMessageNumber = 3,
    endL1L2BridgeRollingHash = "0x04",
    endL1L2BridgeRollingHashMessageNumber = 5,
    dynamicChainConfigHash = "0xc0ffee",
    parentFtxRollingHash = "0x06",
    endFtxRollingHash = "0x07",
    lastProcessedFtxNumber = 8,
    filteredAddressesHash = "0x09",
    txFromsHash = "0x0c",
  )

  private fun expectedExecutionPublicInputs(): L2ExecutionProofPublicInputs = L2ExecutionProofPublicInputs(
    parentBlockHash = "0x0a".decodeHex(),
    endBlockHash = "0x0b".decodeHex(),
    endBlockNumber = 1000503UL,
    endBlockTimestamp = 1763000123UL,
    L2L1MessagesHash = "0x01".decodeHex(),
    parentL1L2BridgeRollingHash = "0x02".decodeHex(),
    parentL1L2BridgeRollingHashMessageNumber = 3UL,
    endL1L2BridgeRollingHash = "0x04".decodeHex(),
    endL1L2BridgeRollingHashMessageNumber = 5UL,
    dynamicChainConfigHash = "0xc0ffee".decodeHex(),
    parentFtxRollingHash = "0x06".decodeHex(),
    endFtxRollingHash = "0x07".decodeHex(),
    lastProcessedFtxNumber = 8UL,
    filteredAddressesHash = "0x09".decodeHex(),
    txFromsHash = "0x0c".decodeHex(),
  )

  private fun rollupPublicInputsDto(): RollupProofPublicInputsDto = RollupProofPublicInputsDto(
    endBlockNumber = 1000520,
    endBlockTimestamp = 1763000457,
    L2L1BridgeTransactionTree = "0x10",
    parentL1L2BridgeRollingHash = "0x11",
    parentL1L2BridgeRollingHashMessageNumber = 12,
    endL1L2BridgeRollingHash = "0x13",
    endL1L2BridgeRollingHashMessageNumber = 14,
    dynamicChainConfigHash = "0xc0ffee",
    parentFtxRollingHash = "0x15",
    endFtxRollingHash = "0x16",
    lastProcessedFtxNumber = 17,
    filteredAddressesHash = "0x18",
    parentShnarf = "0x19",
    endShnarf = "0x1a",
  )

  private fun expectedRollupPublicInputs(): RollupProofPublicInputs = RollupProofPublicInputs(
    endBlockNumber = 1000520UL,
    endBlockTimestamp = Instant.fromEpochSeconds(1763000457L),
    L2L1BridgeTransactionTree = "0x10".decodeHex(),
    parentL1L2BridgeRollingHash = "0x11".decodeHex(),
    parentL1L2BridgeRollingHashMessageNumber = 12UL,
    endL1L2BridgeRollingHash = "0x13".decodeHex(),
    endL1L2BridgeRollingHashMessageNumber = 14UL,
    dynamicChainConfigHash = "0xc0ffee".decodeHex(),
    parentFtxRollingHash = "0x15".decodeHex(),
    endFtxRollingHash = "0x16".decodeHex(),
    lastProcessedFtxNumber = 17UL,
    filteredAddressesHash = "0x18".decodeHex(),
    parentShnarf = "0x19".decodeHex(),
    endShnarf = "0x1a".decodeHex(),
  )

  @Test
  fun `L2ExecutionProofResponseDtoMapper decodes every field`() {
    val dto = L2ExecutionProofResponseDto(
      proof = "0xabcd",
      publicInputs = executionPublicInputsDto(),
      L2L1MsgList = listOf("0xaa"),
      froms = listOf("0xbb"),
      addrs = listOf("0xcc"),
    )

    assertThat(
      L2ExecutionProofResponseDtoMapper(
        ExecutionProofIndex(
          1000500UL,
          1000503UL,
          startBlockTimestamp = Instant.DISTANT_PAST,
        ),
        dto,
      ),
    ).isEqualTo(
      L2ExecutionProofResponse(
        startBlockNumber = 1000500UL,
        endBlockNumber = 1000503UL,
        proof = "0xabcd".decodeHex(),
        publicInputs = expectedExecutionPublicInputs(),
        L2L1MsgList = listOf("0xaa".decodeHex()),
        froms = listOf("0xbb".decodeHex()),
        addrs = listOf("0xcc".decodeHex()),
      ),
    )
  }

  @Test
  fun `RollupProofResponseDtoMapper decodes every field`() {
    val dto = RollupProofResponseDto(
      proof = "0xabcd",
      publicInputs = rollupPublicInputsDto(),
      L2L1Roots = listOf("0xaa"),
      filteredAddresses = listOf("0xbb"),
    )

    assertThat(
      RollupProofResponseDtoMapper(
        CompressionProofIndex(
          1000500UL,
          1000503UL,
          ByteArray(32),
          Instant.DISTANT_PAST,
        ),
        dto,
      ),
    ).isEqualTo(
      RollupProofResponse(
        startBlockNumber = 1000500UL,
        endBlockNumber = 1000503UL,
        proof = "0xabcd".decodeHex(),
        publicInputs = expectedRollupPublicInputs(),
        L2L1Roots = listOf("0xaa".decodeHex()),
        filteredAddresses = listOf("0xbb".decodeHex()),
      ),
    )
  }

  @Test
  fun `RollupAggregationProofResponseDtoMapper decodes every field`() {
    val dto = RollupAggregationProofResponseDto(
      proof = "0xabcd",
      publicInputs = rollupPublicInputsDto(),
    )

    assertThat(
      RollupAggregationProofResponseDtoMapper(
        AggregationProofIndex(
          1000500UL,
          1000503UL,
          ByteArray(32),
          Instant.DISTANT_PAST,
        ),
        dto,
      ),
    ).isEqualTo(
      RollupAggregationProofResponse(
        startBlockNumber = 1000500UL,
        endBlockNumber = 1000503UL,
        proof = "0xabcd".decodeHex(),
        publicInputs = expectedRollupPublicInputs(),
      ),
    )
  }

  @Test
  fun `L2 execution proof response JSON parses into the DTO and maps to the domain response`() {
    val json = """
      {
        "proof": "0xabcd",
        "publicInputs": {
          "parentBlockHash": "0x0a",
          "endBlockHash": "0x0b",
          "endBlockNumber": 1000503,
          "endBlockTimestamp": 1763000123,
          "L2L1MessagesHash": "0x01",
          "parentL1L2BridgeRollingHash": "0x02",
          "parentL1L2BridgeRollingHashMessageNumber": 3,
          "endL1L2BridgeRollingHash": "0x04",
          "endL1L2BridgeRollingHashMessageNumber": 5,
          "dynamicChainConfigHash": "0xc0ffee",
          "parentFtxRollingHash": "0x06",
          "endFtxRollingHash": "0x07",
          "lastProcessedFtxNumber": 8,
          "filteredAddressesHash": "0x09",
          "txFromsHash": "0x0c"
        },
        "L2L1MsgList": ["0xaa"],
        "froms": ["0xbb"],
        "addrs": []
      }
    """.trimIndent()

    val dto = jsonMapper.readValue(json, L2ExecutionProofResponseDto::class.java)

    assertThat(
      L2ExecutionProofResponseDtoMapper(
        ExecutionProofIndex(
          1000500UL,
          1000503UL,
          startBlockTimestamp = Instant.DISTANT_PAST,
        ),
        dto,
      ),
    ).isEqualTo(
      L2ExecutionProofResponse(
        startBlockNumber = 1000500UL,
        endBlockNumber = 1000503UL,
        proof = "0xabcd".decodeHex(),
        publicInputs = expectedExecutionPublicInputs(),
        L2L1MsgList = listOf("0xaa".decodeHex()),
        froms = listOf("0xbb".decodeHex()),
        addrs = emptyList(),
      ),
    )
  }

  @Test
  fun `rollup proof response JSON parses into the DTO and maps to the domain response`() {
    val json = """
      {
        "proof": "0xabcd",
        "publicInputs": {
          "endBlockNumber": 1000520,
          "endBlockTimestamp": 1763000457,
          "L2L1BridgeTransactionTree": "0x10",
          "parentL1L2BridgeRollingHash": "0x11",
          "parentL1L2BridgeRollingHashMessageNumber": 12,
          "endL1L2BridgeRollingHash": "0x13",
          "endL1L2BridgeRollingHashMessageNumber": 14,
          "dynamicChainConfigHash": "0xc0ffee",
          "parentFtxRollingHash": "0x15",
          "endFtxRollingHash": "0x16",
          "lastProcessedFtxNumber": 17,
          "filteredAddressesHash": "0x18",
          "parentShnarf": "0x19",
          "endShnarf": "0x1a"
        },
        "L2L1Roots": ["0xaa"],
        "filteredAddresses": []
      }
    """.trimIndent()

    val dto = jsonMapper.readValue(json, RollupProofResponseDto::class.java)
    val mappedDto = RollupProofResponseDtoMapper(
      CompressionProofIndex(
        1000500UL,
        1000503UL,
        ByteArray(32),
        Instant.DISTANT_PAST,
      ),
      dto,
    )

    assertThat(mappedDto).isEqualTo(
      RollupProofResponse(
        startBlockNumber = 1000500UL,
        endBlockNumber = 1000503UL,
        proof = "0xabcd".decodeHex(),
        publicInputs = expectedRollupPublicInputs(),
        L2L1Roots = listOf("0xaa".decodeHex()),
        filteredAddresses = emptyList(),
      ),
    )
  }
}
