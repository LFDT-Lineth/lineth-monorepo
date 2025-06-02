package net.consensys.linea.transactionexclusion.app

import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.core.LoggerContext
import org.apache.logging.log4j.core.config.Configurator
import picocli.CommandLine
import kotlin.system.exitProcess

class TransactionExclusionAppMain {
  companion object {
    private val log = LogManager.getLogger(TransactionExclusionAppMain::class)

    @JvmStatic
    fun main(args: Array<String>) {
      val cmd = CommandLine(TransactionExclusionAppCli.withAction(::startApp))
      cmd.execute(*args)
    }

    private fun startApp(configs: AppConfig) {
      try {
        val app = TransactionExclusionApp(configs)
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
      } catch (t: Throwable) {
        log.error("Startup failure: ", t)
        exitProcess(1)
      }
    }
  }
}
