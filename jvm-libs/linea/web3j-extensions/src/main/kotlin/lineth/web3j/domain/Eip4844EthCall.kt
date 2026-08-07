package lineth.web3j.domain

import com.fasterxml.jackson.annotation.JsonProperty
import lineth.kotlin.encodeHex
import org.apache.tuweni.bytes.Bytes
import org.web3j.crypto.Blob
import org.web3j.crypto.BlobUtils
import org.web3j.protocol.core.methods.request.Transaction
import org.web3j.utils.Numeric
import tools.jackson.core.JsonGenerator
import tools.jackson.databind.SerializationContext
import tools.jackson.databind.ValueSerializer
import tools.jackson.databind.annotation.JsonSerialize
import java.math.BigInteger
import java.util.*

class Eip4844Transaction(
  from: String,
  nonce: BigInteger?,
  gasPrice: BigInteger?,
  gasLimit: BigInteger?,
  to: String?,
  value: BigInteger?,
  data: String?,
  chainId: Long?,
  maxPriorityFeePerGas: BigInteger?,
  maxFeePerGas: BigInteger?,
  _maxFeePerBlobGas: BigInteger?,
  @JsonProperty("blobs")
  @JsonSerialize(contentUsing = BlobSerializer::class)
  val blobs: List<Blob>,
  @Suppress("Unused")
  @JsonProperty("blobVersionedHashes")
  @JsonSerialize(contentUsing = ByteArrayToHexSerializer::class)
  val blobVersionedHashes: List<ByteArray> = computeVersionedHashesFromBlobs(blobs),
) : Transaction(from, nonce, gasPrice, gasLimit, to, value, data, chainId, maxPriorityFeePerGas, maxFeePerGas) {
  @Suppress("Unused")
  val maxFeePerBlobGas: String? = _maxFeePerBlobGas?.let { Numeric.encodeQuantity(it) }

  companion object {
    fun computeVersionedHashesFromBlobs(blobs: List<Blob>): List<ByteArray> {
      return blobs
        .map(BlobUtils::getCommitment)
        .map(BlobUtils::kzgToVersionedHash)
        .map(Bytes::toArray)
    }

    fun createFunctionCallTransaction(
      from: String,
      to: String,
      data: String,
      blobs: List<Blob>,
      maxFeePerBlobGas: BigInteger? = null,
      gasLimit: BigInteger?,
      blobVersionedHashes: List<ByteArray> = computeVersionedHashesFromBlobs(blobs),
      maxPriorityFeePerGas: BigInteger? = null,
      maxFeePerGas: BigInteger? = null,
    ): Eip4844Transaction {
      return Eip4844Transaction(
        from = from,
        nonce = null,
        gasPrice = null,
        gasLimit = gasLimit,
        to = to,
        value = null,
        data = data,
        chainId = null,
        maxPriorityFeePerGas = maxPriorityFeePerGas,
        maxFeePerGas = maxFeePerGas,
        _maxFeePerBlobGas = maxFeePerBlobGas,
        blobs = blobs,
        blobVersionedHashes = blobVersionedHashes,
      )
    }
  }
}

class BlobSerializer : ValueSerializer<Blob>() {
  override fun serialize(value: Blob, gen: JsonGenerator, ctxt: SerializationContext) {
    gen.writeString(value.data.toHexString().lowercase(Locale.ROOT))
  }
}

class ByteArrayToHexSerializer : ValueSerializer<ByteArray>() {
  override fun serialize(value: ByteArray, gen: JsonGenerator, ctxt: SerializationContext) {
    gen.writeString(value.encodeHex())
  }
}
