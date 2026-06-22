/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package chaos

import chaos.SetupHelper.parseLines
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class SetupHelperTest {
  @Test
  fun `should parse file lines label=value`() {
    val lines =
      listOf(
        "besu-follower-0 = http://127.0.0.1:18545",
        "http://127.0.0.1:28545", // should use url as label if label is empty
        " = http://127.0.0.1:38545", // should use url as label if label is empty
        "    ", // should ignore empty lines
        "  besu-sequencer-0 = http://127.0.0.1:58545",
        "  =  ",
      )

    assertThat(parseLines(lines)).isEqualTo(
      listOf(
        NodeInfo("besu-follower-0", "http://127.0.0.1:18545"),
        NodeInfo("http://127.0.0.1:28545", "http://127.0.0.1:28545"),
        NodeInfo("http://127.0.0.1:38545", "http://127.0.0.1:38545"),
        NodeInfo("besu-sequencer-0", "http://127.0.0.1:58545"),
      ),
    )
  }
}
