package linea.web3j

import com.fasterxml.jackson.annotation.JsonCreator
import com.fasterxml.jackson.annotation.JsonProperty
import linea.domain.FeeHistory
import linea.domain.uLongFromPrefixedHex
import org.web3j.protocol.Web3jService
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.Response
import org.web3j.utils.Numeric
import tools.jackson.core.JsonParser
import tools.jackson.core.JsonToken
import tools.jackson.databind.DeserializationContext
import tools.jackson.databind.ValueDeserializer
import tools.jackson.databind.annotation.JsonDeserialize
import java.math.BigInteger

class EthFeeHistoryBlobExtended : Response<EthFeeHistoryBlobExtended.FeeHistoryBlobExtended>() {
  @JsonDeserialize(using = ResponseDeserializer::class)
  override fun setResult(result: FeeHistoryBlobExtended) {
    super.setResult(result)
  }

  val feeHistory: FeeHistoryBlobExtended
    get() = super.getResult()

  data class FeeHistoryBlobExtended
  @JsonCreator
  constructor(
    @JsonProperty("oldestBlock") val oldestBlock: String,
    @JsonProperty("reward") val reward: List<List<String>>,
    @JsonProperty("baseFeePerGas") val baseFeePerGas: List<String>,
    @JsonProperty("gasUsedRatio") val gasUsedRatio: List<Double>,
    @JsonProperty("baseFeePerBlobGas") val baseFeePerBlobGas: List<String>,
    @JsonProperty("blobGasUsedRatio") val blobGasUsedRatio: List<Double>,
  ) {
    override fun equals(other: Any?): Boolean {
      if (this === other) return true
      if (javaClass != other?.javaClass) return false

      other as FeeHistoryBlobExtended

      if (oldestBlock != other.oldestBlock) return false
      if (reward != other.reward) return false
      if (baseFeePerGas != other.baseFeePerGas) return false
      if (gasUsedRatio != other.gasUsedRatio) return false
      if (baseFeePerBlobGas != other.baseFeePerBlobGas) return false
      return blobGasUsedRatio == other.blobGasUsedRatio
    }

    override fun hashCode(): Int {
      var result = oldestBlock.hashCode()
      result = 31 * result + reward.hashCode()
      result = 31 * result + baseFeePerGas.hashCode()
      result = 31 * result + gasUsedRatio.hashCode()
      result = 31 * result + baseFeePerBlobGas.hashCode()
      result = 31 * result + blobGasUsedRatio.hashCode()
      return result
    }

    fun toLineaDomain(): FeeHistory {
      return FeeHistory(
        oldestBlock = oldestBlock.uLongFromPrefixedHex(),
        baseFeePerGas = baseFeePerGas.map(String::uLongFromPrefixedHex),
        reward = reward.map { it.map(String::uLongFromPrefixedHex) },
        gasUsedRatio = gasUsedRatio,
        baseFeePerBlobGas = baseFeePerBlobGas.map(String::uLongFromPrefixedHex),
        blobGasUsedRatio = blobGasUsedRatio,
      )
    }
  }

  class ResponseDeserializer : ValueDeserializer<FeeHistoryBlobExtended>() {
    override fun deserialize(
      jsonParser: JsonParser,
      deserializationContext: DeserializationContext,
    ): FeeHistoryBlobExtended? {
      return if (jsonParser.currentToken() != JsonToken.VALUE_NULL) {
        // delegate through the context (not a standalone ObjectReader) because jsonParser is
        // positioned mid-stream inside the enclosing Response object: an ObjectReader.readValue(parser)
        // treats the read as a whole document and trips FAIL_ON_TRAILING_TOKENS (default true in
        // Jackson 3) on the outer object's closing token.
        deserializationContext.readValue(jsonParser, FeeHistoryBlobExtended::class.java)
      } else {
        null
      }
    }
  }
}

class Web3jBlobExtended(private val web3jService: Web3jService) {
  fun ethFeeHistoryWithBlob(
    blockCount: Int,
    newestBlock: DefaultBlockParameter,
    rewardPercentiles: List<Double>,
  ): Request<*, EthFeeHistoryBlobExtended> {
    return Request(
      "eth_feeHistory",
      listOf(
        Numeric.encodeQuantity(BigInteger.valueOf(blockCount.toLong())),
        newestBlock.value,
        rewardPercentiles,
      ),
      this.web3jService,
      EthFeeHistoryBlobExtended::class.java,
    )
  }
}
