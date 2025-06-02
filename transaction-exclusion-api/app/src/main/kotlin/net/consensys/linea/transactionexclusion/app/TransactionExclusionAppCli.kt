package net.consensys.linea.transactionexclusion.app

import com.sksamuel.hoplite.ConfigFailure
import com.sksamuel.hoplite.ConfigLoaderBuilder
import com.sksamuel.hoplite.addFileSource
import com.sksamuel.hoplite.fp.Validated
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import picocli.CommandLine.Command
import picocli.CommandLine.Parameters
import java.io.File
import java.io.PrintWriter
import java.nio.charset.Charset
import java.util.concurrent.Callable

@Command(
  name = TransactionExclusionAppCli.COMMAND_NAME,
  showDefaultValues = true,
  abbreviateSynopsis = true,
  description = ["Runs Transaction Exclusion API Service"],
  version = ["0.0.1"],
  synopsisHeading = "%n",
  descriptionHeading = "%nDescription:%n%n",
  optionListHeading = "%nOptions:%n",
  footerHeading = "%n",
)
class TransactionExclusionAppCli
internal constructor(private val errorWriter: PrintWriter, private val startAction: StartAction) :
  Callable<Int> {
  @Parameters(paramLabel = "CONFIG.toml", description = ["Configuration files"])
  private val configFiles: List<File>? = null

  override fun call(): Int {
    return try {
      if (configFiles == null) {
        errorWriter.println("Please provide a configuration file!")
        printUsage(errorWriter)
        return 1
      }
      for (configFile in configFiles) {
        if (!canReadFile(configFile)) {
          return 1
        }
      }
      val configs: Validated<ConfigFailure, AppConfig> = configs(configFiles)
      if (configs.isInvalid()) {
        errorWriter.println(configs.getInvalidUnsafe().description())
        return 1
      }
      startAction.start(configs.getUnsafe())
      0
    } catch (e: Exception) {
      reportUserError(e)
      1
    }
  }

  private fun canReadFile(file: File): Boolean {
    if (!file.canRead()) {
      errorWriter.println("Cannot read configuration file '${file.absolutePath}'")
      return false
    }
    return true
  }

  fun configs(configFiles: List<File>): Validated<ConfigFailure, AppConfig> {
    val confBuilder: ConfigLoaderBuilder = ConfigLoaderBuilder.Companion.empty().addDefaults()
    for (i in configFiles.indices.reversed()) {
      // files must be added in reverse order for overriding

      // files must be added in reverse order for overriding
      confBuilder.addFileSource(configFiles[i], false)
    }
    val config: Validated<ConfigFailure, AppConfig> =
      confBuilder.build().loadConfig<AppConfig>(emptyList())

    return config
  }

  fun reportUserError(ex: Throwable) {
    logger.fatal(ex.message, ex)
    errorWriter.println(ex.message)
    printUsage(errorWriter)
  }

  private fun printUsage(outputWriter: PrintWriter) {
    outputWriter.println()
    outputWriter.println("To display full help:")
    outputWriter.println(COMMAND_NAME + " --help")
  }

  /**
   * Not using a static field for this log instance because some code in this class executes prior
   * to the logging configuration being applied so it's not always safe to use the logger.
   *
   * Where this is used we also ensure the messages are printed to the error writer so they will be
   * printed even if logging is not yet configured.
   *
   * @return the logger for this class
   */
  private val logger: Logger = LogManager.getLogger()

  fun interface StartAction {
    fun start(configs: AppConfig)
  }

  companion object {
    const val COMMAND_NAME = "transaction-exclusion"
    fun withAction(startAction: StartAction): TransactionExclusionAppCli {
      val errorWriter = PrintWriter(System.err, true, Charset.defaultCharset())
      return TransactionExclusionAppCli(errorWriter, startAction)
    }
  }
}
