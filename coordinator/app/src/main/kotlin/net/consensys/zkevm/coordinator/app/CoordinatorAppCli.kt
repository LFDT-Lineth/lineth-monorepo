package net.consensys.zkevm.coordinator.app

import linea.coordinator.config.v2.CoordinatorConfig
import linea.coordinator.config.v2.toml.loadConfigs
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import picocli.CommandLine
import picocli.CommandLine.ArgGroup
import picocli.CommandLine.Command
import picocli.CommandLine.Parameters
import java.io.File
import java.io.PrintWriter
import java.nio.charset.Charset
import java.util.concurrent.Callable

@Command(
  name = CoordinatorAppCli.COMMAND_NAME,
  showDefaultValues = true,
  abbreviateSynopsis = true,
  description = ["Runs Linea Coordinator"],
  version = ["0.0.1"],
  synopsisHeading = "%n",
  descriptionHeading = "%nDescription:%n%n",
  optionListHeading = "%nOptions:%n",
  footerHeading = "%n",
)
class CoordinatorAppCli
internal constructor(private val errorWriter: PrintWriter, private val startAction: StartAction) :
  Callable<Int> {
  @Parameters(paramLabel = "CONFIG.toml", description = ["Configuration files"])
  private val configFiles: List<File>? = null

  @ArgGroup(multiplicity = "1", exclusive = true, validate = true)
  private var tracesLimitsFiles: TracesLimitsFiles? = null

  class TracesLimitsFiles {
    @CommandLine.Option(
      names = ["--traces-limits-v4"],
      paramLabel = "<FILE>",
      description = ["Prover traces limits V4 for linea besu"],
      required = false,
    )
    var tracesLimitsV4File: File? = null

    @CommandLine.Option(
      names = ["--traces-limits-v5"],
      paramLabel = "<FILE>",
      description = ["Prover traces limits V5 for linea besu"],
      required = false,
    )
    var tracesLimitsV5File: File? = null
  }

  @CommandLine.Option(
    names = ["--smart-contract-errors"],
    paramLabel = "<FILE>",
    description = ["Smart contract error codes"],
    arity = "1",
  )
  private val smartContractErrorsFile: File? = null

  @CommandLine.Option(
    names = ["--gas-price-cap-time-of-day-multipliers"],
    paramLabel = "<FILE>",
    description = ["Time-of-day multipliers for calculation of L1 dynamic gas price caps"],
    arity = "1",
  )
  private val gasPriceCapTimeOfDayMultipliersFile: File? = null

  @CommandLine.Option(
    names = ["--check-configs-only"],
    paramLabel = "<BOOLEAN>",
    description = ["Validates configuration files only, without starting the application."],
    arity = "0..1",
  )
  private var checkConfigsOnly: Boolean = false

  override fun call(): Int {
    return try {
      if (configFiles == null) {
        errorWriter.println("Please provide a configuration file!")
        printUsage(errorWriter)
        return 1
      }
      if (
        tracesLimitsFiles!!.tracesLimitsV4File == null &&
        tracesLimitsFiles!!.tracesLimitsV5File == null
      ) {
        errorWriter.println("Please provide a traces-limits file!")
        printUsage(errorWriter)
        return 1
      }
      if (smartContractErrorsFile == null) {
        errorWriter.println("Please provide smart-contract-errors file!")
        printUsage(errorWriter)
        return 1
      }
      if (gasPriceCapTimeOfDayMultipliersFile == null) {
        errorWriter.println("Please provide gas-price-cap-time-of-day-multipliers file!")
        printUsage(errorWriter)
        return 1
      }

      for (configFile in configFiles) {
        if (!canReadFile(configFile)) {
          return 1
        }
      }

      val configs =
        loadConfigs(
          coordinatorConfigFiles = configFiles.map { it.toPath() },
          tracesLimitsFileV4 = tracesLimitsFiles!!.tracesLimitsV4File?.toPath(),
          tracesLimitsFileV5 = tracesLimitsFiles!!.tracesLimitsV5File?.toPath(),
          smartContractErrorsFile = smartContractErrorsFile.toPath(),
          gasPriceCapTimeOfDayMultipliersFile = gasPriceCapTimeOfDayMultipliersFile.toPath(),
          logger = logger,
        )

      if (checkConfigsOnly) {
        logger.info("All configs are valid. Final configs: {}", configs)
      } else {
        startAction.start(configs)
      }
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

  fun reportUserError(ex: Throwable) {
    logger.fatal(ex.message, ex)
    errorWriter.println(ex.message)
    printUsage(errorWriter)
  }

  private fun printUsage(outputWriter: PrintWriter) {
    outputWriter.println()
    outputWriter.println("To display full help:")
    outputWriter.println("$COMMAND_NAME --help")
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
    fun start(configs: CoordinatorConfig)
  }

  companion object {
    const val COMMAND_NAME = "coordinator"

    fun withAction(startAction: StartAction): CoordinatorAppCli {
      val errorWriter = PrintWriter(System.err, true, Charset.defaultCharset())
      return CoordinatorAppCli(errorWriter, startAction)
    }
  }
}
