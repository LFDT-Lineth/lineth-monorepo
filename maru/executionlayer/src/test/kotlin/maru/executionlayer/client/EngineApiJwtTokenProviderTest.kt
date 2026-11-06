/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.client

import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Files
import java.nio.file.Path
import java.time.Clock
import java.time.Instant
import java.time.ZoneOffset
import java.util.Base64

class EngineApiJwtTokenProviderTest {
  @TempDir
  lateinit var dir: Path

  private val secretHex = "0f".repeat(32)
  private val clock = Clock.fixed(Instant.ofEpochSecond(1_700_000_000), ZoneOffset.UTC)

  private fun secretFile(content: String): Path = Files.writeString(dir.resolve("jwt.hex"), content)

  private fun decodePart(token: String, index: Int) = String(Base64.getUrlDecoder().decode(token.split(".")[index]))

  @Test
  fun `token carries HS256 header and current iat`() {
    val token = EngineApiJwtTokenProvider(ByteArray(32), clock).token()

    assertThat(token.split(".")).hasSize(3)
    assertThat(decodePart(token, 0)).isEqualTo("""{"alg":"HS256","typ":"JWT"}""")
    assertThat(decodePart(token, 1)).isEqualTo("""{"iat":1700000000}""")
  }

  @Test
  fun `loads secrets with or without 0x prefix and surrounding whitespace`() {
    val expected = EngineApiJwtTokenProvider(ByteArray(32) { 0x0f }, clock).token()

    assertThat(EngineApiJwtTokenProvider.fromSecretFile(secretFile(secretHex), clock).token()).isEqualTo(expected)
    assertThat(EngineApiJwtTokenProvider.fromSecretFile(secretFile("  0x$secretHex\n"), clock).token())
      .isEqualTo(expected)
  }

  @Test
  fun `rejects missing, non-hex and wrongly sized secrets`() {
    assertThatThrownBy { EngineApiJwtTokenProvider.fromSecretFile(dir.resolve("missing.hex")) }
      .isInstanceOf(java.nio.file.NoSuchFileException::class.java)
    assertThatThrownBy { EngineApiJwtTokenProvider.fromSecretFile(secretFile("zz".repeat(32))) }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("does not contain a valid hex secret")
    assertThatThrownBy { EngineApiJwtTokenProvider.fromSecretFile(secretFile("0f".repeat(16))) }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessage("JWT secret must be 32 bytes, got 16")
  }
}
