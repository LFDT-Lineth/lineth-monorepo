package net.consensys.linea.vertx

import io.vertx.config.ConfigRetriever
import io.vertx.config.ConfigRetrieverOptions
import io.vertx.config.ConfigStoreOptions
import io.vertx.core.Vertx
import io.vertx.core.VertxOptions
import io.vertx.core.json.JsonObject
import net.consensys.linea.async.get

fun loadVertxConfig(): VertxOptions {
  val vertx: Vertx = Vertx.vertx()
  val sysPropsStore = ConfigStoreOptions().setType("sys")
  val options = ConfigRetrieverOptions()
  options.addStore(sysPropsStore)

  val vertxPropertiesConfigFile = System.getProperty("vertx.configurationFile")
  if (vertxPropertiesConfigFile != null) {
    val jsonRetrieverOptions = ConfigStoreOptions().setType("file").setConfig(
      JsonObject()
        .put("hierarchical", true)
        .put("path", vertxPropertiesConfigFile)
    )
    options.addStore(jsonRetrieverOptions)
  }

  // Close the vert.x instance, we don't need it anymore.
  val retriever = ConfigRetriever.create(vertx, options)
  val parsedOptions = VertxOptions(retriever.config.get())
  vertx.close().get()
  return parsedOptions
}
