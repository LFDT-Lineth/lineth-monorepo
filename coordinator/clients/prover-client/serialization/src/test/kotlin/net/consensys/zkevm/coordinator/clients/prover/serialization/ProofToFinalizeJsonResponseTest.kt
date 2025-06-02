package net.consensys.zkevm.coordinator.clients.prover.serialization

import com.fasterxml.jackson.core.JsonToken
import com.fasterxml.jackson.databind.ObjectMapper
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.assertDoesNotThrow
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.Arguments
import org.junit.jupiter.params.provider.MethodSource
import java.io.File
import java.nio.file.Path
import java.util.stream.Stream
import kotlin.reflect.full.declaredMemberProperties

class ProofToFinalizeJsonResponseTest {

  @ParameterizedTest(name = "when_deserialize_{0}_does_not_throw")
  @MethodSource("aggregationProofResponseFiles")
  fun when_deserialize_test_data_does_not_throw(filePath: Path) {
    assertDoesNotThrow {
      JsonSerialization.proofResponseMapperV1.readValue(filePath.toFile(), ProofToFinalizeJsonResponse::class.java)
        .toDomainObject()
    }
  }

  @ParameterizedTest(name = "when_deserialize_{0}_properties_match_proof_to_finalize_json_response")
  @MethodSource("aggregationProofResponseFiles")
  fun when_deserialize_test_data_json_properties_match_proof_to_finalize_json_response(
    filePath: Path,
  ) {
    val proofToFinalizeJsonResponseProperties = ProofToFinalizeJsonResponse::class
      .declaredMemberProperties
      .map { it.name }
      .toMutableSet()

    proofToFinalizeJsonResponseProperties.addAll(ProofToFinalizeJsonResponse.PROPERTIES_NOT_INCLUDED)

    val propertiesInTestDataFile = getKeysInJsonUsingJsonParser(
      filePath.toFile(),
      JsonSerialization.proofResponseMapperV1,
    )
    assertThat(propertiesInTestDataFile).isEqualTo(proofToFinalizeJsonResponseProperties)
  }

  companion object {
    private const val testDataPath = "../../../../testdata/prover/prover-aggregation/responses"

    @JvmStatic
    fun aggregationProofResponseFiles(): Stream<Arguments> {
      return File(testDataPath).listFiles()!!
        .filter { it.isFile && it.name.endsWith(".json") }
        .map { filePath ->
          Arguments.of(filePath.toPath())
        }.stream()
    }

    private fun getKeysInJsonUsingJsonParser(jsonFile: File, mapper: ObjectMapper): Set<String> {
      val keys: MutableSet<String> = HashSet()
      val jsonNode = mapper.readTree(jsonFile)
      val jsonParser = jsonNode.traverse()
      while (!jsonParser.isClosed) {
        if (jsonParser.nextToken() == JsonToken.FIELD_NAME) {
          keys.add(jsonParser.currentName())
        }
      }
      return keys
    }
  }
}
