package net.consensys.zkevm.coordinator.app

import linea.coordinator.config.v2.CoordinatorConfig
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
      val app = CoordinatorApp(configs)
      Runtime.getRuntime()
        .addShutdownHook(
          Thread {
            app.stop()
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
