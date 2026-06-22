package linea.persistence.db

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.RepeatedTest
import java.time.Clock
import java.time.Instant
import java.time.ZoneId

class DbHelperTest {
  @RepeatedTest(1)
  fun `creates random db name`() {
    val clock = Clock.fixed(Instant.parse("2023-02-01T10:11:22.33Z"), ZoneId.of("UTC"))
    val dbName = DbHelper.generateUniqueDbName("test-some-dao", clock)
    assertThat(dbName).matches("test_some_dao_20230201101122_[0-9a-f]{8}")
  }
}
