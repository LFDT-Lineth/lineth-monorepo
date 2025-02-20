package net.consensys.zkevm.load

import com.fasterxml.jackson.core.JsonParser
import com.fasterxml.jackson.core.JsonToken
import com.fasterxml.jackson.databind.DeserializationContext
import com.fasterxml.jackson.databind.JsonDeserializer
import com.fasterxml.jackson.databind.ObjectReader
import com.fasterxml.jackson.databind.annotation.JsonDeserialize
import linea.domain.bigIntFromPrefixedHex
import org.web3j.protocol.ObjectMapperFactory
import org.web3j.protocol.core.Response
import java.io.IOException
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
        priorityFeePerGas = result!!.priorityFeePerGas.bigIntFromPrefixedHex()
      )
    } else {
      null
    }
  }

  data class GasEstimation(val baseFeePerGas: BigInteger, val gasLimit: BigInteger, val priorityFeePerGas: BigInteger)
  data class GasEstimationSerialized(val baseFeePerGas: String, val gasLimit: String, val priorityFeePerGas: String) {
    constructor() : this("", "", "")

    companion object {
      class ResponseDeserialiser : JsonDeserializer<GasEstimationSerialized?>() {
        private val objectReader: ObjectReader = ObjectMapperFactory.getObjectReader()

        @Throws(IOException::class)
        override fun deserialize(
          jsonParser: JsonParser,
          deserializationContext: DeserializationContext
        ): GasEstimationSerialized? {
          return if (jsonParser.currentToken != JsonToken.VALUE_NULL) {
            objectReader.readValue(jsonParser, GasEstimationSerialized::class.java)
          } else {
            null // null is wrapped by Optional in above getter
          }
        }
      }
    }
  }
}
