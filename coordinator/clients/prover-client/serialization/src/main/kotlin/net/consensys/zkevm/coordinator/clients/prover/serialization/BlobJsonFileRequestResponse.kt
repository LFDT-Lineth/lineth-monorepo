package net.consensys.zkevm.coordinator.clients.prover.serialization

import com.fasterxml.jackson.annotation.JsonProperty
import com.fasterxml.jackson.core.JsonGenerator
import com.fasterxml.jackson.core.JsonParser
import com.fasterxml.jackson.databind.DeserializationContext
import com.fasterxml.jackson.databind.JsonDeserializer
import com.fasterxml.jackson.databind.JsonSerializer
import com.fasterxml.jackson.databind.SerializerProvider
import com.fasterxml.jackson.databind.annotation.JsonDeserialize
import com.fasterxml.jackson.databind.annotation.JsonSerialize
import linea.domain.BlockIntervals
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import net.consensys.zkevm.coordinator.clients.BlobCompressionProof
import net.consensys.zkevm.coordinator.clients.BlobCompressionProofRequest

internal class ByteArrayDeserializer : JsonDeserializer<ByteArray>() {
  override fun deserialize(p: JsonParser, ctxt: DeserializationContext): ByteArray {
    return p.valueAsString.decodeHex()
  }
}

internal class ByteArraySerializer : JsonSerializer<ByteArray>() {
  override fun serialize(value: ByteArray, gen: JsonGenerator, serializers: SerializerProvider) {
    gen.writeString(value.encodeHex())
  }
}

data class BlobCompressionProofJsonRequest(
  val compressedData: ByteArray,
  val conflationOrder: BlockIntervals,
  @JsonProperty("prevShnarf")
  @JsonSerialize(using = ByteArraySerializer::class)
  val prevShnarf: ByteArray,
  @JsonProperty("parentStateRootHash")
  @JsonSerialize(using = ByteArraySerializer::class)
  val parentStateRootHash: ByteArray,
  @JsonProperty("finalStateRootHash")
  @JsonSerialize(using = ByteArraySerializer::class)
  val finalStateRootHash: ByteArray,
  @JsonProperty("parentDataHash")
  @JsonSerialize(using = ByteArraySerializer::class)
  val parentDataHash: ByteArray,
  @JsonProperty("dataHash")
  @JsonSerialize(using = ByteArraySerializer::class)
  val dataHash: ByteArray,
  @JsonProperty("snarkHash")
  @JsonSerialize(using = ByteArraySerializer::class)
  val snarkHash: ByteArray,
  @JsonProperty("expectedX")
  @JsonSerialize(using = ByteArraySerializer::class)
  val expectedX: ByteArray,
  @JsonProperty("expectedY")
  @JsonSerialize(using = ByteArraySerializer::class)
  val expectedY: ByteArray,
  @JsonProperty("expectedShnarf")
  @JsonSerialize(using = ByteArraySerializer::class)
  val expectedShnarf: ByteArray,
  val eip4844Enabled: Boolean = true,
  @JsonProperty("commitment")
  @JsonSerialize(using = ByteArraySerializer::class)
  val commitment: ByteArray,
  @JsonProperty("kzgProofContract")
  @JsonSerialize(using = ByteArraySerializer::class)
  val kzgProofContract: ByteArray,
  @JsonProperty("kzgProofSidecar")
  @JsonSerialize(using = ByteArraySerializer::class)
  val kzgProofSidecar: ByteArray,
) {
  companion object {
    fun fromDomainObject(
      request: BlobCompressionProofRequest,
    ): BlobCompressionProofJsonRequest {
      return BlobCompressionProofJsonRequest(
        compressedData = request.compressedData,
        conflationOrder = BlockIntervals(
          startingBlockNumber = request.conflations.first().startBlockNumber,
          upperBoundaries = request.conflations.map { it.endBlockNumber },
        ),
        prevShnarf = request.prevShnarf,
        parentStateRootHash = request.parentStateRootHash,
        finalStateRootHash = request.finalStateRootHash,
        parentDataHash = request.parentDataHash,
        dataHash = request.expectedShnarfResult.dataHash,
        snarkHash = request.expectedShnarfResult.snarkHash,
        expectedX = request.expectedShnarfResult.expectedX,
        expectedY = request.expectedShnarfResult.expectedY,
        expectedShnarf = request.expectedShnarfResult.expectedShnarf,
        commitment = request.commitment,
        kzgProofContract = request.kzgProofContract,
        kzgProofSidecar = request.kzgProofSideCar,
      )
    }
  }
}

data class BlobCompressionProofJsonResponse(
  val compressedData: ByteArray, // The data that are explicitly sent in the blob (i.e. after compression)
  val conflationOrder: BlockIntervals,
  @JsonProperty("prevShnarf")
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  @JsonSerialize(using = ByteArraySerializer::class)
  val prevShnarf: ByteArray,
  @JsonProperty("parentStateRootHash")
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  @JsonSerialize(using = ByteArraySerializer::class)
  val parentStateRootHash: ByteArray, // Parent root hash
  @JsonProperty("finalStateRootHash")
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  @JsonSerialize(using = ByteArraySerializer::class)
  val finalStateRootHash: ByteArray, // New state root hash
  @JsonProperty("parentDataHash")
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  @JsonSerialize(using = ByteArraySerializer::class)
  val parentDataHash: ByteArray,
  @JsonProperty("dataHash")
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  @JsonSerialize(using = ByteArraySerializer::class)
  val dataHash: ByteArray,
  @JsonProperty("snarkHash")
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  @JsonSerialize(using = ByteArraySerializer::class)
  val snarkHash: ByteArray, // The snarkHash used for the blob consistency check
  @JsonProperty("expectedX")
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  @JsonSerialize(using = ByteArraySerializer::class)
  val expectedX: ByteArray,
  @JsonProperty("expectedY")
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  @JsonSerialize(using = ByteArraySerializer::class)
  val expectedY: ByteArray,
  @JsonProperty("expectedShnarf")
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  @JsonSerialize(using = ByteArraySerializer::class)
  val expectedShnarf: ByteArray,
  @JsonProperty("decompressionProof")
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  @JsonSerialize(using = ByteArraySerializer::class)
  val decompressionProof: ByteArray, // zkProof of compression and consistency
  val proverVersion: String,
  val verifierID: Long,
  val eip4844Enabled: Boolean = true,
  @JsonProperty("commitment")
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  @JsonSerialize(using = ByteArraySerializer::class)
  val commitment: ByteArray = ByteArray(0),
  @JsonProperty("kzgProofContract")
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  @JsonSerialize(using = ByteArraySerializer::class)
  val kzgProofContract: ByteArray = ByteArray(0),
  @JsonProperty("kzgProofSidecar")
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  @JsonSerialize(using = ByteArraySerializer::class)
  val kzgProofSidecar: ByteArray = ByteArray(0),
) {

  fun toDomainObject(): BlobCompressionProof {
    return BlobCompressionProof(
      compressedData = compressedData,
      conflationOrder = conflationOrder,
      prevShnarf = prevShnarf,
      parentStateRootHash = parentStateRootHash,
      finalStateRootHash = finalStateRootHash,
      parentDataHash = parentDataHash,
      dataHash = dataHash,
      snarkHash = snarkHash,
      expectedX = expectedX,
      expectedY = expectedY,
      expectedShnarf = expectedShnarf,
      decompressionProof = decompressionProof,
      proverVersion = proverVersion,
      verifierID = verifierID,
      commitment = commitment,
      kzgProofContract = kzgProofContract,
      kzgProofSidecar = kzgProofSidecar,
    )
  }

  fun toJsonString(): String {
    return JsonSerialization.proofResponseMapperV1.writeValueAsString(this)
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as BlobCompressionProofJsonResponse

    if (!compressedData.contentEquals(other.compressedData)) return false
    if (conflationOrder != other.conflationOrder) return false
    if (!prevShnarf.contentEquals(other.prevShnarf)) return false
    if (!parentStateRootHash.contentEquals(other.parentStateRootHash)) return false
    if (!finalStateRootHash.contentEquals(other.finalStateRootHash)) return false
    if (!parentDataHash.contentEquals(other.parentDataHash)) return false
    if (!dataHash.contentEquals(other.dataHash)) return false
    if (!snarkHash.contentEquals(other.snarkHash)) return false
    if (!expectedX.contentEquals(other.expectedX)) return false
    if (!expectedY.contentEquals(other.expectedY)) return false
    if (!expectedShnarf.contentEquals(other.expectedShnarf)) return false
    if (!decompressionProof.contentEquals(other.decompressionProof)) return false
    if (proverVersion != other.proverVersion) return false
    if (verifierID != other.verifierID) return false
    if (eip4844Enabled != other.eip4844Enabled) return false
    if (!commitment.contentEquals(other.commitment)) return false
    if (!kzgProofContract.contentEquals(other.kzgProofContract)) return false
    if (!kzgProofSidecar.contentEquals(other.kzgProofSidecar)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = compressedData.contentHashCode()
    result = 31 * result + conflationOrder.hashCode()
    result = 31 * result + prevShnarf.contentHashCode()
    result = 31 * result + parentStateRootHash.contentHashCode()
    result = 31 * result + finalStateRootHash.contentHashCode()
    result = 31 * result + parentDataHash.contentHashCode()
    result = 31 * result + dataHash.contentHashCode()
    result = 31 * result + snarkHash.contentHashCode()
    result = 31 * result + expectedX.contentHashCode()
    result = 31 * result + expectedY.contentHashCode()
    result = 31 * result + expectedShnarf.contentHashCode()
    result = 31 * result + decompressionProof.contentHashCode()
    result = 31 * result + proverVersion.hashCode()
    result = 31 * result + verifierID.hashCode()
    result = 31 * result + eip4844Enabled.hashCode()
    result = 31 * result + commitment.contentHashCode()
    result = 31 * result + kzgProofContract.contentHashCode()
    result = 31 * result + kzgProofSidecar.contentHashCode()
    return result
  }

  companion object {
    fun fromJsonString(jsonString: String): BlobCompressionProofJsonResponse {
      return JsonSerialization.proofResponseMapperV1.readValue(
        jsonString,
        BlobCompressionProofJsonResponse::class.java,
      )
    }

    fun fromDomainObject(compressionProof: BlobCompressionProof): BlobCompressionProofJsonResponse {
      return BlobCompressionProofJsonResponse(
        compressedData = compressionProof.compressedData,
        conflationOrder = compressionProof.conflationOrder,
        prevShnarf = compressionProof.prevShnarf,
        parentStateRootHash = compressionProof.parentStateRootHash,
        finalStateRootHash = compressionProof.finalStateRootHash,
        parentDataHash = compressionProof.parentDataHash,
        dataHash = compressionProof.dataHash,
        snarkHash = compressionProof.snarkHash,
        expectedX = compressionProof.expectedX,
        expectedY = compressionProof.expectedY,
        expectedShnarf = compressionProof.expectedShnarf,
        decompressionProof = compressionProof.decompressionProof,
        proverVersion = compressionProof.proverVersion,
        verifierID = compressionProof.verifierID,
        commitment = compressionProof.commitment,
        kzgProofContract = compressionProof.kzgProofContract,
        kzgProofSidecar = compressionProof.kzgProofSidecar,
      )
    }
  }
}
