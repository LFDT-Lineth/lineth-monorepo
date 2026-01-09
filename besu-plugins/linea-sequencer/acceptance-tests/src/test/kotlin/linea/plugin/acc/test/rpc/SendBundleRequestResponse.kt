package linea.plugin.acc.test.rpc

import com.fasterxml.jackson.core.JsonParser
import com.fasterxml.jackson.core.JsonToken
import com.fasterxml.jackson.databind.DeserializationContext
import com.fasterxml.jackson.databind.JsonDeserializer
import com.fasterxml.jackson.databind.ObjectReader
import com.fasterxml.jackson.databind.annotation.JsonDeserialize
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import linea.plugin.acc.test.rpc.linea.AbstractSendBundleTest.BundleParams
import linea.plugin.acc.test.utils.toLogString
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.tests.acceptance.dsl.transaction.NodeRequests
import org.hyperledger.besu.tests.acceptance.dsl.transaction.Transaction
import org.web3j.protocol.core.Request
import java.io.IOException

class SendBundleRequest(private val bundleParams: BundleParams) :
  Transaction<SendBundleResponse> {

  override fun execute(nodeRequests: NodeRequests): SendBundleResponse {
    return try {
      Request(
        "linea_sendBundle",
        listOf(bundleParams),
        nodeRequests.web3jService,
        SendBundleResponse::class.java,
      ).send()
    } catch (e: IOException) {
      throw RuntimeException(e)
    }
  }
}

class SendBundleResponse : org.web3j.protocol.core.Response<SendBundleResponse.SendBundleResponseData>() {

  @Override
  @JsonDeserialize(using = SendBundleResponseDeserializer::class)
  override fun setResult(result: SendBundleResponseData) {
    super.setResult(result)
  }

  data class SendBundleResponseData(val bundleHash: String)
}

class SendBundleResponseDeserializer : JsonDeserializer<SendBundleResponse.SendBundleResponseData>() {
  private val objectReader: ObjectReader = jacksonObjectMapper().reader()

  override fun deserialize(
    jsonParser: JsonParser,
    deserializationContext: DeserializationContext,
  ): SendBundleResponse.SendBundleResponseData? {
    return if (jsonParser.currentToken != JsonToken.VALUE_NULL) {
      objectReader.readValue(jsonParser, SendBundleResponse.SendBundleResponseData::class.java)
    } else {
      null
    }
  }
}

fun SendBundleResponse.assertSuccessResponse() {
  assertThat(this.error)
    .withFailMessage { this.error?.toLogString() ?: "no error" }
    .isNull()
  assertThat(this.result.bundleHash).isNotBlank()
}
