package net.consensys.zkevm.load.model.inner

import net.consensys.zkevm.load.model.JSON
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertNotNull
import org.junit.jupiter.api.Test
import java.io.InputStreamReader
import kotlin.test.assertFailsWith

class RequestTest {

  @Test
  fun translateCreateContractWithoutGasLimitFailsWithClearError() {
    // Arrange
    val requestJson =
      """
      {
        "id": 1,
        "name": "deploy contract",
        "context": {
          "chainId": 59141,
          "url": "https://rpc.sepolia.linea.build",
          "nbOfExecutions": 1
        },
        "calls": [
          {
            "nbOfExecution": 1,
            "scenario": {
              "scenarioType": "ContractCall",
              "wallet": "source",
              "contract": {
                "contractCallType": "CreateContract",
                "name": "dummyContract",
                "byteCode": "0x00"
              }
            }
          }
        ]
      }
      """.trimIndent()

    val swaggerRequest =
      JSON.createGson().create().fromJson(requestJson, net.consensys.zkevm.load.swagger.Request::class.java)

    // Act
    val exception = assertFailsWith<IllegalArgumentException> {
      Request.translate(swaggerRequest)
    }

    // Assert
    assertEquals("CreateContract `dummyContract` is missing required gasLimit.", exception.message)
  }

  @Test
  fun translateSepoliaLoadTestRequestSucceeds() {
    // Arrange
    val swaggerRequest = loadRequestFromResource("sepolia/load-test.json")

    // Act
    val request = Request.translate(swaggerRequest)

    // Assert
    assertNotNull(request)
  }

  @Test
  fun translateLocalLoadTestRequestSucceeds() {
    // Arrange
    val swaggerRequest = loadRequestFromResource("local/load-test.json")

    // Act
    val request = Request.translate(swaggerRequest)

    // Assert
    assertNotNull(request)
  }

  @Test
  fun translateLocalDeployContractRequestSucceeds() {
    // Arrange
    val swaggerRequest = loadRequestFromResource("local/deploy-contract.json")

    // Act
    val request = Request.translate(swaggerRequest)

    // Assert
    assertNotNull(request)
  }

  @Test
  fun translateDevnetDeployContractRequestSucceeds() {
    // Arrange
    val swaggerRequest = loadRequestFromResource("devnet/deploy-contract.json")

    // Act
    val request = Request.translate(swaggerRequest)

    // Assert
    assertNotNull(request)
  }

  private fun loadRequestFromResource(path: String): net.consensys.zkevm.load.swagger.Request {
    val resource = checkNotNull(this.javaClass.classLoader.getResource(path))
    return JSON.createGson().create().fromJson(
      InputStreamReader(resource.openStream()),
      net.consensys.zkevm.load.swagger.Request::class.java,
    )
  }
}
