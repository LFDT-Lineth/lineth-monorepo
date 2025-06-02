package net.consensys.linea.logging

import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.apache.logging.log4j.core.LoggerContext
import org.apache.logging.log4j.core.test.appender.ListAppender
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.TestInstance
import org.junit.jupiter.api.assertThrows

@TestInstance(TestInstance.Lifecycle.PER_CLASS)
class Log4JLineaAppenderTest {
  private lateinit var listAppender: ListAppender
  private lateinit var logger: Logger

  private val knownErrorMessage = "A known error"
  private val knownErrorWithStackTraceMessage = "Unsuppressed stack trace error"
  private val unknownErrorMessage = "An unknown error"

  @BeforeEach
  fun setUp() {
    logger = LogManager.getLogger(Log4JLineaAppenderTest::class.java)
    val ctx = LogManager.getContext(false) as LoggerContext
    val config = ctx.configuration
    listAppender = config.getAppender("ListAppender") as ListAppender
  }

  @Test
  fun `known error triggers rewrite policy and logs at expected level`() {
    logger.error(knownErrorMessage)

    val logEvent = listAppender.events.last()
    assertEquals(knownErrorMessage, logEvent.message.formattedMessage)
  }

  @Test
  fun `unknown error doesn't trigger rewrite policy and is unchanged`() {
    logger.error(unknownErrorMessage)

    val logEvent = listAppender.events.last()
    assertEquals(Level.ERROR, logEvent.level)
    assertEquals(unknownErrorMessage, logEvent.message.formattedMessage)
  }

  @Test
  fun `known thrown error has stack trace suppressed`() {
    val exception = assertThrows<IllegalStateException> {
      throw IllegalStateException(knownErrorMessage)
    }

    logger.error(
      "errorMessage={}",
      exception.message,
      exception,
    )
    val logEvent = listAppender.events.last()

    assertEquals(Level.WARN, logEvent.level)
    assertEquals("errorMessage=$knownErrorMessage", logEvent.message.formattedMessage)
  }

  @Test
  fun `known thrown error with unsuppressed stack trace`() {
    val exception = assertThrows<IllegalStateException> {
      throw IllegalStateException(knownErrorWithStackTraceMessage)
    }

    logger.error(
      "errorMessage={}",
      exception.message,
      exception,
    )
    val logEvent = listAppender.events.last()

    assertEquals(Level.WARN, logEvent.level)
    assertEquals("errorMessage=$knownErrorWithStackTraceMessage", logEvent.message.formattedMessage)
    assertThat(exception.stackTrace.contentEquals(logEvent.thrown.stackTrace)).isTrue()
  }

  @Test
  fun `unknown thrown error doesn't have stack trace suppressed`() {
    val exception = assertThrows<IllegalStateException> {
      throw IllegalStateException(unknownErrorMessage)
    }

    logger.error(
      "errorMessage={}",
      exception.message,
      exception,
    )
    val logEvent = listAppender.events.last()

    assertEquals(Level.ERROR, logEvent.level)
    assertEquals("errorMessage=$unknownErrorMessage", logEvent.message.formattedMessage)
    assertThat(exception.stackTrace.contentEquals(logEvent.thrown.stackTrace)).isTrue()
  }

  @Test
  fun `if error is eth_call log level is rewritten`() {
    logger.error(
      "eth_call for aggregation finalization failed Contract Call has been reverted by the EVM with the reason",
    )
    val logEvent = listAppender.events.last()

    assertEquals(Level.INFO, logEvent.level)
  }

  @Test
  fun `if error is not an eth_call log level remains at ERROR level`() {
    logger.error(
      "aggregation finalization failed Contract Call has been reverted by the EVM with the reason",
    )
    val logEvent = listAppender.events.last()

    assertEquals(Level.ERROR, logEvent.level)
  }

  @Test
  fun `regex matches with mismatched cases`() {
    logger.error("a KnOwN eRRor")
    val logEvent = listAppender.events.last()
    assertEquals(Level.WARN, logEvent.level)
  }
}
