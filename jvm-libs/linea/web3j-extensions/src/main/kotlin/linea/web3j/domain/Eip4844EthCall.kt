package linea.web3j.domain

import build.linea.s11n.jackson.ByteArrayToHexSerializer
import com.fasterxml.jackson.annotation.JsonProperty
import com.fasterxml.jackson.core.JsonGenerator
import com.fasterxml.jackson.databind.JsonSerializer
import com.fasterxml.jackson.databind.SerializerProvider
import com.fasterxml.jackson.databind.annotation.JsonSerialize
import org.apache.tuweni.bytes.Bytes
import org.web3j.crypto.Blob
import org.web3j.crypto.BlobUtils
import org.web3j.protocol.core.methods.request.Transaction
import org.web3j.utils.Numeric
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
  val blobVersionedHashes: List<ByteArray> = computeVersionedHashesFromBlobs(blobs)
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
      maxFeePerGas: BigInteger? = null
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
        blobVersionedHashes = blobVersionedHashes
      )
    }
  }
}

class BlobSerializer : JsonSerializer<Blob>() {
  override fun serialize(value: Blob, gen: JsonGenerator, provider: SerializerProvider) {
    gen.writeString(value.data.toHexString().lowercase(Locale.getDefault()))
  }
}
