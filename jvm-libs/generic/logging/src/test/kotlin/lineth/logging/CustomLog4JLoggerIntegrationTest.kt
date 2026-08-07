package lineth.logging

import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.apache.logging.log4j.core.LoggerContext
import org.apache.logging.log4j.core.test.appender.ListAppender
import org.assertj.core.api.Assertions
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.TestInstance

@TestInstance(TestInstance.Lifecycle.PER_CLASS)
class CustomLog4JLoggerIntegrationTest {
  // Configuration is defined in file log4j2.xml in test resources
  private lateinit var listAppender: ListAppender
  private lateinit var log: Logger

  @BeforeEach
  fun setUp() {
    log = LogManager.getLogger(Log4JLineaAppenderTest::class.java)
    val ctx = LogManager.getContext(false) as LoggerContext
    val config = ctx.configuration
    listAppender = config.getAppender("ListAppender") as ListAppender
  }

  @AfterEach
  fun afterEach() {
    listAppender.clear()
  }

  @Test
  fun `DebouncingFilter is configurable with resources`() {
    val loggedString = "AAAAAA!"
    for (i in 1..10) {
      log.info(loggedString)
    }

    Assertions.assertThat(listAppender.events).size().isEqualTo(1)
  }

  @Test
  fun `Known messages are rewritten and also debounced`() {
    val loggedString = "A known error!"
    for (i in 1..10) {
      log.error(loggedString)
    }

    val latestLogEvent = listAppender.events.last()
    Assertions.assertThat(listAppender.events).size().isEqualTo(1)
    Assertions.assertThat(latestLogEvent.message.formattedMessage).isEqualTo(loggedString)
    Assertions.assertThat(latestLogEvent.level).isEqualTo(Level.WARN)
  }

  @Test
  fun `Unknown messages are rewritten and also debounced`() {
    val loggedString = "This log is unknown error!"
    for (i in 1..10) {
      log.info(loggedString)
    }

    val latestLogEvent = listAppender.events.last()
    Assertions.assertThat(listAppender.events).size().isEqualTo(1)
    Assertions.assertThat(latestLogEvent.message.formattedMessage).isEqualTo(loggedString)
    Assertions.assertThat(latestLogEvent.level).isEqualTo(Level.INFO)
  }
}
