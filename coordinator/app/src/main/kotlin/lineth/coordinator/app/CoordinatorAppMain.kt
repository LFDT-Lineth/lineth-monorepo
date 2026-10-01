package lineth.coordinator.app

import io.vertx.core.Vertx
import lineth.coordinator.config.v2.CoordinatorConfig
import net.consensys.linea.async.get
import net.consensys.linea.vertx.loadVertxConfig
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.core.LoggerContext
import org.apache.logging.log4j.core.config.Configurator
import picocli.CommandLine
import kotlin.system.exitProcess

class CoordinatorAppMain {
  companion object {
    private val log = LogManager.getLogger(CoordinatorAppMain::class)

    @JvmStatic
    fun main(args: Array<String>) {
      val cmd = CommandLine(CoordinatorAppCli.withAction(::startApp))
      cmd.setExecutionExceptionHandler { ex, _, _ ->
        log.error("Execution failure: ", ex)
        1
      }
      cmd.setParameterExceptionHandler { ex, _ ->
        log.error("Invalid args!: ", ex)
        1
      }
      val exitCode = cmd.execute(*args)
      if (exitCode != 0) {
        exitProcess(exitCode)
      }
    }

    private fun startApp(configs: CoordinatorConfig) {
      val vertxConfig = loadVertxConfig()
      log.trace("System properties: {}", System.getProperties())
      log.debug("Vertx full configs: {}", vertxConfig)
      val vertx = Vertx.vertx(loadVertxConfig())
      val app = CoordinatorApp(configs, vertx = vertx)
      Runtime.getRuntime()
        .addShutdownHook(
          Thread {
            app.stop()
            vertx.close().get()
            log.info("vertx Stopped")
            if (LogManager.getContext() is LoggerContext) {
              // Disable log4j auto shutdown hook is not used otherwise
              // Messages in App.stop won't appear in the logs
              Configurator.shutdown(LogManager.getContext() as LoggerContext)
            }
          },
        )
      app.start()
    }
  }
}
