package net.consensys.linea.web3j

import com.fasterxml.jackson.core.JsonParser
import com.fasterxml.jackson.core.JsonToken
import com.fasterxml.jackson.databind.DeserializationContext
import com.fasterxml.jackson.databind.JsonDeserializer
import com.fasterxml.jackson.databind.ObjectReader
import com.fasterxml.jackson.databind.annotation.JsonDeserialize
import linea.domain.FeeHistory
import linea.domain.uLongFromPrefixedHex
import org.web3j.protocol.ObjectMapperFactory
import org.web3j.protocol.Web3jService
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.Response
import org.web3j.utils.Numeric
import java.math.BigInteger

class EthFeeHistoryBlobExtended : Response<EthFeeHistoryBlobExtended.FeeHistoryBlobExtended>() {
  @JsonDeserialize(using = ResponseDeserializer::class)
  override fun setResult(result: FeeHistoryBlobExtended) {
    super.setResult(result)
  }

  val feeHistory: FeeHistoryBlobExtended
    get() = super.getResult()

  data class FeeHistoryBlobExtended(
    val oldestBlock: String,
    val reward: List<List<String>>,
    val baseFeePerGas: List<String>,
    val gasUsedRatio: List<Double>,
    val baseFeePerBlobGas: List<String>,
    val blobGasUsedRatio: List<Double>
  ) {
    constructor() : this(
      oldestBlock = "",
      reward = emptyList(),
      baseFeePerGas = emptyList(),
      gasUsedRatio = emptyList(),
      baseFeePerBlobGas = emptyList(),
      blobGasUsedRatio = emptyList()
    )

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
        blobGasUsedRatio = blobGasUsedRatio
      )
    }
  }

  class ResponseDeserializer : JsonDeserializer<FeeHistoryBlobExtended>() {
    private val objectReader: ObjectReader = ObjectMapperFactory.getObjectReader()

    override fun deserialize(
      jsonParser: JsonParser,
      deserializationContext: DeserializationContext
    ): FeeHistoryBlobExtended? {
      return if (jsonParser.currentToken != JsonToken.VALUE_NULL) {
        objectReader.readValue(jsonParser, FeeHistoryBlobExtended::class.java)
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
    rewardPercentiles: List<Double>
  ): Request<*, EthFeeHistoryBlobExtended> {
    return Request(
      "eth_feeHistory",
      listOf(
        Numeric.encodeQuantity(BigInteger.valueOf(blockCount.toLong())),
        newestBlock.value,
        rewardPercentiles
      ),
      this.web3jService,
      EthFeeHistoryBlobExtended::class.java
    )
  }
}
