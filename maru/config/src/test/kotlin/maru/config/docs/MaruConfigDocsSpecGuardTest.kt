/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.config.docs

import maru.config.MaruConfigDtoToml
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

/**
 * Guards against the regression fixed in PR #3735: the config-docs spec must root at the Hoplite
 * TOML schema ([MaruConfigDtoToml] — the type [maru.config.MaruConfigLoader.loadAppConfigs]
 * returns), not the domain [maru.config.MaruConfig]. `checkConfigDocs` only verifies
 * annotation coverage of whichever class the spec names, so it cannot detect a class mismatch;
 * this test does.
 */
class MaruConfigDocsSpecGuardTest {
  @Test
  fun `spec roots at the Hoplite TOML DTO, not the domain config`() {
    val rootClasses = MaruConfigDocsSpec.files.map { it.rootClass }
    assertThat(rootClasses).containsExactly(MaruConfigDtoToml::class)
  }
}
