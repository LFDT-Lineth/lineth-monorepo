/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test

class MaruAppResourcesTest {
  @Test
  fun `close attempts every resource and preserves failures`() {
    val firstFailure = IllegalStateException("p2p close failed")
    val secondFailure = IllegalArgumentException("signer close failed")
    val closedResources = mutableListOf<String>()

    assertThatThrownBy {
      closeAll(
        { throw firstFailure },
        { closedResources += "vertx" },
        { throw secondFailure },
        { closedResources += "database" },
      )
    }.isSameAs(firstFailure)

    assertThat(closedResources).containsExactly("vertx", "database")
    assertThat(firstFailure.suppressed).containsExactly(secondFailure)
  }
}
