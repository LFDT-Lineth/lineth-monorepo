package linea.staterecovery.clients.blobscan

import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock
import com.github.tomakehurst.wiremock.core.WireMockConfiguration
import com.github.tomakehurst.wiremock.http.RequestListener
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import net.consensys.encodeHex
import net.consensys.linea.async.get
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import java.net.URI
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

@ExtendWith(VertxExtension::class)
class BlobScanClientTest {

  private lateinit var wiremock: WireMockServer
  private lateinit var meterRegistry: SimpleMeterRegistry
  private lateinit var serverURI: URI
  private lateinit var blobScanClient: BlobScanClient

  @BeforeEach
  fun setUp(vertx: Vertx) {
    wiremock = WireMockServer(
      WireMockConfiguration.options()
        .dynamicPort()
    )
      .apply {
        addMockServiceRequestListener(object : RequestListener {
          override fun requestReceived(
            request: com.github.tomakehurst.wiremock.http.Request,
            response: com.github.tomakehurst.wiremock.http.Response
          ) {
            // to debug
            // println("request: ${request.url}")
          }
        })
      }
    wiremock.start()
    meterRegistry = SimpleMeterRegistry()

    serverURI = URI("http://127.0.0.1:${wiremock.port()}")
    blobScanClient = BlobScanClient.create(
      vertx = vertx,
      endpoint = URI(wiremock.baseUrl()),
      requestRetryConfig = RequestRetryConfig(
        backoffDelay = 10.milliseconds,
        maxRetries = 5u,
        timeout = 5.seconds
      )
    )
  }

  @AfterEach
  fun tearDown(vertx: Vertx) {
    val vertxStopFuture = vertx.close()
    wiremock.stop()
    vertxStopFuture.get()
  }

  @Test
  fun `when blobs exists shall return it`() {
    val blobId = "0x0139f94e70bbbc39c821459ccd74245ff34212d76077df454d490f76790d563c"
    val blobData = "0x0006eac4e2fac2ca844810be0dc9e398fa4961656c022b65"

    wiremock.stubFor(
      WireMock.get("/blobs/$blobId")
        .withHeader("Accept", WireMock.containing("application/json"))
        .willReturn(
          WireMock.ok()
            .withHeader("Content-type", "application/json")
            .withBody(successResponseBody(blobId, blobData))
        )
    )

    assertThat(blobScanClient.getBlobById(blobId).get().encodeHex()).isEqualTo(blobData)
  }

  @Test
  fun `when blobs does not exists shall return error message`() {
    val blobId = "0x0139f94e70bbbc39c821459ccd74245ff34212d76077df454d490f76790d563c"

    wiremock.stubFor(
      WireMock.get("/blobs/$blobId")
        .withHeader("Accept", WireMock.containing("application/json"))
        .willReturn(
          WireMock
            .notFound()
            .withHeader("Content-type", "application/json")
            .withBody(
              """
               {"message":"No blob with versioned hash or kzg commitment '$blobId'.","code":"NOT_FOUND"}
              """.trimIndent()
            )
        )
    )

    assertThatThrownBy { blobScanClient.getBlobById(blobId).get() }
      .hasMessageContaining("No blob with versioned hash or kzg commitment")
  }

  @Test
  fun `when request failed shall retry it`() {
    val blobId = "0x0139f94e70bbbc39c821459ccd74245ff34212d76077df454d490f76790d563c"
    val blobData = "0x0006eac4e2fac2ca844810be0dc9e398fa4961656c022b65"

    wiremock.stubFor(
      WireMock.get("/blobs/$blobId")
        .inScenario("SERVER_ERROR")
        .willReturn(WireMock.status(503))
        .willSetStateTo("SERVER_ERROR_1")
    )
    wiremock.stubFor(
      WireMock.get("/blobs/$blobId")
        .inScenario("SERVER_ERROR")
        .whenScenarioStateIs("SERVER_ERROR_1")
        .willReturn(WireMock.status(503))
        .willSetStateTo("SERVER_OK")
    )
    wiremock.stubFor(
      WireMock.get("/blobs/$blobId")
        .inScenario("SERVER_ERROR")
        .whenScenarioStateIs("SERVER_OK")
        .willReturn(
          WireMock.okJson(successResponseBody(blobId, blobData))
        )
    )

    assertThat(blobScanClient.getBlobById(blobId).get().encodeHex()).isEqualTo(blobData)
  }

  private fun successResponseBody(
    blobId: String,
    blobData: String
  ): String {
    return """
      {
        "commitment": "0x86cddad176d1db92ac521c5dada895e1cca048a86618f131f271f54f07130daddd51af1f416be7ede789f6305d00d670",
        "proof": "0x8ec34bdd70967eaa212b8c16c783f48940d6d0ab402b410290fd709511adb86a219bae75a41f295f0d7c6b0e22a74c38",
        "size": 131072,
        "versionedHash": "$blobId",
        "data": "$blobData",
        "dataStorageReferences": [
          {
            "blobStorage": "google",
            "dataReference": "1/01/39/f9/0139f94e70bbbc39c821459ccd74245ff34212d76077df454d490f76790d563c.txt"
          },
          {
            "blobStorage": "swarm",
            "dataReference": "ff6758f14e3becd98f4a38588ff6371a4669aedbb4fd17604b501781b7646b41"
          }
        ],
        "transactions": [
          {
            "hash": "0xe085a55b76df624824948f5611b363c3b23c8ff5db92df327004c8e66282227e",
            "index": 0,
            "blockHash": "0x5bc4cd40c9af4f3ec3d260b46c907c05f62f68ec12fc6b893a827d8cf8043b41",
            "blockNumber": 20860690,
            "blockTimestamp": "2024-09-30T03:11:23.000Z",
            "rollup": "linea"
          }
        ]
      }
    """.trimIndent()
  }
}
