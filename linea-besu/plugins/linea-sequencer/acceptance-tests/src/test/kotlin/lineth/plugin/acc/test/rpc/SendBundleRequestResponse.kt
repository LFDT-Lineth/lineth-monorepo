package lineth.plugin.acc.test.rpc

import com.fasterxml.jackson.annotation.JsonCreator
import com.fasterxml.jackson.annotation.JsonProperty
import lineth.plugin.acc.test.rpc.linea.AbstractSendBundleTest.BundleParams
import lineth.plugin.acc.test.utils.toLogString
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

  // web3j's Service deserializes responses with its own internal Jackson databind engine (as of
  // web3j 5.0.3, that's Jackson 3's tools.jackson.databind, not com.fasterxml.jackson.databind),
  // which has no knowledge of Kotlin data class constructors. jackson-annotations (JsonCreator/
  // JsonProperty) is the one Jackson module shared unchanged across the 2.x/3.x split, so
  // annotating the constructor directly (rather than registering a custom JsonDeserializer via
  // @JsonDeserialize, whose com.fasterxml annotation type Jackson 3's engine doesn't recognize)
  // works regardless of which databind engine ends up parsing the response.
  data class SendBundleResponseData
  @JsonCreator
  constructor(@JsonProperty("bundleHash") val bundleHash: String)
}

fun SendBundleResponse.assertSuccessResponse() {
  assertThat(this.error)
    .withFailMessage { this.error?.toLogString() ?: "no error" }
    .isNull()
  assertThat(this.result.bundleHash).isNotBlank()
}
