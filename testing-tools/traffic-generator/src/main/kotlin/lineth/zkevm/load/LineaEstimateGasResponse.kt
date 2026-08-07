package lineth.zkevm.load

import com.fasterxml.jackson.annotation.JsonCreator
import com.fasterxml.jackson.annotation.JsonProperty
import lineth.domain.bigIntFromPrefixedHex
import org.web3j.protocol.ObjectMapperFactory
import org.web3j.protocol.core.Response
import tools.jackson.core.JsonParser
import tools.jackson.core.JsonToken
import tools.jackson.databind.DeserializationContext
import tools.jackson.databind.ObjectReader
import tools.jackson.databind.ValueDeserializer
import tools.jackson.databind.annotation.JsonDeserialize
import java.math.BigInteger

/** eth_feeHistory.  */
class LineaEstimateGasResponse : Response<LineaEstimateGasResponse.GasEstimationSerialized?>() {
  @JsonDeserialize(using = GasEstimationSerialized.Companion.ResponseDeserialiser::class)
  override fun setResult(result: GasEstimationSerialized?) {
    super.setResult(result)
  }

  fun getGasEstimation(): GasEstimation? {
    return if (result != null) {
      GasEstimation(
        baseFeePerGas = result!!.baseFeePerGas.bigIntFromPrefixedHex(),
        gasLimit = result!!.gasLimit.bigIntFromPrefixedHex(),
        priorityFeePerGas = result!!.priorityFeePerGas.bigIntFromPrefixedHex(),
      )
    } else {
      null
    }
  }

  data class GasEstimation(val baseFeePerGas: BigInteger, val gasLimit: BigInteger, val priorityFeePerGas: BigInteger)
  data class GasEstimationSerialized
  @JsonCreator
  constructor(
    @JsonProperty("baseFeePerGas") val baseFeePerGas: String,
    @JsonProperty("gasLimit") val gasLimit: String,
    @JsonProperty("priorityFeePerGas") val priorityFeePerGas: String,
  ) {
    companion object {
      class ResponseDeserialiser : ValueDeserializer<GasEstimationSerialized?>() {
        private val objectReader: ObjectReader = ObjectMapperFactory.getObjectReader()
          .forType(GasEstimationSerialized::class.java)

        override fun deserialize(
          jsonParser: JsonParser,
          deserializationContext: DeserializationContext,
        ): GasEstimationSerialized? {
          return if (jsonParser.currentToken() != JsonToken.VALUE_NULL) {
            objectReader.readValue(jsonParser)
          } else {
            null // null is wrapped by Optional in above getter
          }
        }
      }
    }
  }
}
