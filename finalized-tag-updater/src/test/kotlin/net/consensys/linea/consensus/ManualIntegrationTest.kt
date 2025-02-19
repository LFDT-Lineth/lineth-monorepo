package net.consensys.linea.consensus

import io.vertx.core.Vertx
import linea.consensus.EngineBlockTagUpdater
import net.consensys.linea.LineaL1FinalizationUpdaterService
import net.consensys.linea.PluginConfig
import net.consensys.linea.async.get
import org.hyperledger.besu.datatypes.Address
import java.net.URI
import kotlin.time.Duration.Companion.seconds

class FakeEngineBlockTagUpdater : EngineBlockTagUpdater {
  override fun lineaUpdateFinalizedBlockV1(
    finalizedBlockNumber: Long
  ) {
    println("Linea finalized block update: blockNumber=$finalizedBlockNumber")
  }
}

fun main() {
  val infuraAppKey = System.getenv("INFURA_APP_KEY")
  val vertx = Vertx.vertx()
  val config = PluginConfig(
    l1RpcEndpoint = URI("https://mainnet.infura.io/v3/$infuraAppKey").toURL(),
    l1SmartContractAddress = Address.fromHexString("0xd19d4B5d358258f05D7B411E21A1460D11B0876F"),
    l1PollingInterval = 1.seconds
  )
  val service = LineaL1FinalizationUpdaterService(vertx, config, FakeEngineBlockTagUpdater())
  service.start().get()
  println("service started")
  Thread.sleep(10000)
  service.stop().get()
  vertx.close().get()
}
