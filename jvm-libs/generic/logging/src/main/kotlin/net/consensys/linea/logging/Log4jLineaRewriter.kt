package net.consensys.linea.logging

import org.apache.logging.log4j.Level
import org.apache.logging.log4j.core.LogEvent
import org.apache.logging.log4j.core.appender.rewrite.RewritePolicy
import org.apache.logging.log4j.core.config.plugins.Plugin
import org.apache.logging.log4j.core.config.plugins.PluginAttribute
import org.apache.logging.log4j.core.config.plugins.PluginElement
import org.apache.logging.log4j.core.config.plugins.PluginFactory
import org.apache.logging.log4j.core.impl.Log4jLogEvent

@Plugin(name = "Log4jLineaRewriter", category = "Core", elementType = "rewritePolicy", printObject = true)
class Log4jLineaRewriter(
  private val knownErrors: KnownErrors,
) : RewritePolicy {
  companion object {
    @PluginFactory
    @JvmStatic
    fun createPolicy(
      @PluginElement(value = "knownErrors")
      knownErrorsConfig: KnownErrorsConfig,
    ): Log4jLineaRewriter {
      return Log4jLineaRewriter(
        KnownErrors(
          knownErrorsConfig.knownErrors.toList().map {
            KnownError(Level.getLevel(it.logLevel), it.messagePattern.toRegex(RegexOption.IGNORE_CASE), it.stackTrace)
          },
        ),
      )
    }
  }

  override fun rewrite(event: LogEvent): LogEvent {
    val knownError = knownErrors.find(event.message.formattedMessage)

    return if (knownError != null) {
      val error = event.thrown
      val builder = Log4jLogEvent.Builder(event)
      val stackTrace = suppressStackTraceIfKnownError(error, knownError.stackTrace)
      builder.setThrown(stackTrace)
      builder.setLevel(knownError.logLevel)
      builder.build()
    } else {
      event
    }
  }

  private fun suppressStackTraceIfKnownError(error: Throwable?, includeStackTrace: Boolean): Throwable? {
    return if (!includeStackTrace) {
      null
    } else {
      error
    }
  }
}

@Plugin(name = "KnownErrors", category = "Core", printObject = true)
class KnownErrorsConfig(val knownErrors: Array<KnownErrorConfig>) {

  companion object {
    @PluginFactory
    @JvmStatic
    fun createKnownErrorsConfig(
      @PluginElement("KnownError")
      knownErrors: Array<KnownErrorConfig>,
    ): KnownErrorsConfig {
      return KnownErrorsConfig(knownErrors)
    }
  }
}

@Plugin(name = "KnownError", category = "Core", printObject = true)
class KnownErrorConfig(
  val logLevel: String,
  val messagePattern: String,
  val stackTrace: Boolean = false,
) {
  companion object {
    @PluginFactory
    @JvmStatic
    fun createKnownErrorConfig(
      @PluginAttribute("logLevel") logLevel: String,
      @PluginAttribute("message") message: String,
      @PluginAttribute("stackTrace") stackTrace: Boolean?,
    ): KnownErrorConfig {
      return KnownErrorConfig(
        logLevel = logLevel,
        messagePattern = message,
        stackTrace = stackTrace ?: false,
      )
    }
  }
}
