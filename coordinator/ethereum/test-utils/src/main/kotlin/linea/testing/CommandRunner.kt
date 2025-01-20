package linea.testing

import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.testing.filesystem.getPathTo
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.io.BufferedReader
import java.io.File
import java.io.InputStreamReader
import kotlin.time.Duration
import kotlin.time.Duration.Companion.minutes

data class CommandResult(
  val exitCode: Int,
  val stdOut: List<String>,
  val stdErr: List<String>
)

object Runner {

  fun executeCommand(
    command: String,
    envVars: Map<String, String> = emptyMap(),
    executionDir: File = getPathTo("Makefile").parent.toFile(),
    timeout: Duration = 1.minutes
  ): SafeFuture<CommandResult> {
    val log = LogManager.getLogger("net.consensys.zkevm.ethereum.CommandExecutor")
    val processBuilder = ProcessBuilder("/bin/sh", "-c", command)
    processBuilder.directory(executionDir)

    // Set environment variables
    val env = processBuilder.environment()
    for ((key, value) in envVars) {
      env[key] = value
    }

    val process = processBuilder.start()
    val stdOutReader = BufferedReader(InputStreamReader(process.inputStream))
    val stdErrorReader = BufferedReader(InputStreamReader(process.errorStream))

    // Read the standard output
    log.debug(
      "going to execute command: dir='{}', command='{}', envVars={} commandProcessId={} processInfo={}",
      executionDir,
      command,
      envVars,
      process.pid(),
      process.info()
    )
    process.waitFor(timeout.inWholeMilliseconds, java.util.concurrent.TimeUnit.MILLISECONDS)
    val futureResult = process
      .onExit()
      .thenApply { processResult ->
        val stdOutLines = stdOutReader.lines().toList()
        val stdErrLines = stdErrorReader.lines().toList()
        log.debug(
          "command finished: dir='{}', command='{}', exitCode={} envVars={} processId={} threadId={}",
          executionDir,
          command,
          processResult.exitValue(),
          envVars,
          ProcessHandle.current().pid(),
          Thread.currentThread().threadId()
        )
        log.debug(
          "stdout: {}",
          stdOutLines.joinToString("\n")
        )
        log.debug(
          "stderr: {}",
          stdErrLines.joinToString("\n")
        )
        CommandResult(processResult.exitValue(), stdOutLines, stdErrLines)
      }

    return futureResult.toSafeFuture()
  }
}
