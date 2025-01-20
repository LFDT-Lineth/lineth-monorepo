package linea.staterecovery.clients.el

import build.linea.s11n.jackson.InstantAsHexNumberDeserializer
import build.linea.s11n.jackson.InstantAsHexNumberSerializer
import build.linea.s11n.jackson.ethApiObjectMapper
import com.fasterxml.jackson.annotation.JsonInclude
import com.fasterxml.jackson.databind.JsonNode
import com.fasterxml.jackson.databind.module.SimpleModule
import kotlinx.datetime.Instant
import linea.staterecovery.BlockFromL1RecoveredData
import linea.staterecovery.ExecutionLayerClient
import linea.staterecovery.StateRecoveryStatus
import net.consensys.decodeHex
import net.consensys.fromHexString
import net.consensys.linea.BlockNumberAndHash
import net.consensys.linea.BlockParameter
import net.consensys.linea.jsonrpc.client.JsonRpcClientFactory
import net.consensys.linea.jsonrpc.client.JsonRpcV2Client
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.net.URI

class ExecutionLayerJsonRpcClient internal constructor(
  private val rpcClient: JsonRpcV2Client
) : ExecutionLayerClient {
  override fun getBlockNumberAndHash(blockParameter: BlockParameter): SafeFuture<BlockNumberAndHash> {
    return rpcClient
      .makeRequest(
        method = "eth_getBlockByNumber",
        params = listOf(blockParameter.toString(), false)
      ) { result: Any? ->
        @Suppress("UNCHECKED_CAST")
        result as JsonNode
        BlockNumberAndHash(
          number = ULong.fromHexString(result.get("number").asText()),
          hash = result.get("hash").asText().decodeHex()
        )
      }
  }

  override fun lineaEngineImportBlocksFromBlob(blocks: List<BlockFromL1RecoveredData>): SafeFuture<Unit> {
    return rpcClient
      .makeRequest(
        method = "linea_importBlocksFromBlob",
        params = blocks,
        resultMapper = { Unit }
      )
  }

  override fun lineaGetStateRecoveryStatus(): SafeFuture<StateRecoveryStatus> {
    return rpcClient
      .makeRequest(
        method = "linea_getStateRecoveryStatus",
        params = emptyList<Unit>(),
        resultMapper = Companion::stateRecoveryStatusFromJsonNode
      )
  }

  override fun lineaEnableStateRecovery(stateRecoverStartBlockNumber: ULong): SafeFuture<StateRecoveryStatus> {
    return rpcClient
      .makeRequest(
        method = "linea_enableStateRecovery",
        params = listOf(stateRecoverStartBlockNumber),
        resultMapper = Companion::stateRecoveryStatusFromJsonNode
      )
  }

  companion object {
    fun stateRecoveryStatusFromJsonNode(result: Any?): StateRecoveryStatus {
      @Suppress("UNCHECKED_CAST")
      result as JsonNode
      return StateRecoveryStatus(
        headBlockNumber = ULong.fromHexString(result.get("headBlockNumber").asText()),
        stateRecoverStartBlockNumber = result.get("recoveryStartBlockNumber")
          ?.let { if (it.isNull) null else ULong.fromHexString(it.asText()) }
      )
    }

    fun create(
      rpcClientFactory: JsonRpcClientFactory,
      endpoint: URI,
      requestRetryConfig: RequestRetryConfig,
      logger: Logger = LogManager.getLogger(ExecutionLayerJsonRpcClient::class.java)
    ): ExecutionLayerClient {
      val objectManager = ethApiObjectMapper
        .copy()
        .registerModules(
          SimpleModule().apply {
            this.addSerializer(Instant::class.java, InstantAsHexNumberSerializer)
            this.addDeserializer(Instant::class.java, InstantAsHexNumberDeserializer)
          }
        )
        .setSerializationInclusion(JsonInclude.Include.NON_NULL)
      return ExecutionLayerJsonRpcClient(
        rpcClient = rpcClientFactory.createJsonRpcV2Client(
          endpoints = listOf(endpoint),
          retryConfig = requestRetryConfig,
          requestObjectMapper = objectManager,
          log = logger
        )
      )
    }
  }
}
