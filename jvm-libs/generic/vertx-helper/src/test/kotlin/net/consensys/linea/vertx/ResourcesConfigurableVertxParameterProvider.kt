package net.consensys.linea.vertx

import io.vertx.core.buffer.Buffer
import io.vertx.core.json.JsonObject
import io.vertx.junit5.VertxParameterProvider
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import java.nio.file.Paths

class ResourcesConfigurableVertxParameterProvider : VertxParameterProvider() {
  private val log: Logger = LogManager.getLogger(this::class.java)
  private val VERTX_PARAMETER_FILENAME_SYS_PROP: String = "vertx.parameter.filename"

  override fun getVertxOptions(): JsonObject {
    val optionFileName: String = System.getProperty(VERTX_PARAMETER_FILENAME_SYS_PROP) ?: return JsonObject()
    try {
      val path = Paths.get(optionFileName)
      val content =
        Buffer.buffer(this::class.java.getResourceAsStream(path.toString())!!.readAllBytes())
      return JsonObject(content)
    } catch (e: Exception) {
      log.warn(
        "Failure when reading Vert.x options file {} from property {}, will use default options",
        optionFileName,
        VERTX_PARAMETER_FILENAME_SYS_PROP,
        e,
      )
      return JsonObject()
    }
  }
}
