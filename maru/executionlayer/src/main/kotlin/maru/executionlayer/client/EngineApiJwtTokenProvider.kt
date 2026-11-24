/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.client

import linea.kotlin.decodeHex
import org.apache.logging.log4j.LogManager
import java.io.IOException
import java.nio.file.NoSuchFileException
import java.nio.file.Path
import java.time.Clock
import java.util.Base64
import javax.crypto.Mac
import javax.crypto.spec.SecretKeySpec
import kotlin.io.path.readText

/**
 * Issues Engine API JWTs as described in
 * https://github.com/ethereum/execution-apis/blob/main/src/engine/authentication.md.
 *
 * Every call signs a new HS256 token whose `iat` is the current time, so tokens never go stale between requests.
 */
class EngineApiJwtTokenProvider(
  secret: ByteArray,
  private val clock: Clock = Clock.systemUTC(),
) {
  private val key: SecretKeySpec

  init {
    require(secret.size == SECRET_SIZE) { "JWT secret must be $SECRET_SIZE bytes, got ${secret.size}" }
    key = SecretKeySpec(secret.copyOf(), HMAC_ALGORITHM)
  }

  fun token(): String {
    val claims = base64Url("""{"iat":${clock.instant().epochSecond}}""".toByteArray())
    val signingInput = "$ENCODED_HEADER.$claims"
    val signature = Mac.getInstance(HMAC_ALGORITHM).apply { init(key) }.doFinal(signingInput.toByteArray())
    return "$signingInput.${base64Url(signature)}"
  }

  fun authorizationHeader(): Map<String, String> = mapOf("Authorization" to "Bearer ${token()}")

  companion object {
    private const val SECRET_SIZE = 32
    private const val HMAC_ALGORITHM = "HmacSHA256"
    private val base64UrlEncoder = Base64.getUrlEncoder().withoutPadding()
    private val log = LogManager.getLogger(EngineApiJwtTokenProvider::class.java)
    private val ENCODED_HEADER = base64Url("""{"alg":"HS256","typ":"JWT"}""".toByteArray())

    private fun base64Url(bytes: ByteArray): String = base64UrlEncoder.encodeToString(bytes)

    /** Reads a hex-encoded secret, with or without `0x` prefix, as written by execution clients. */
    fun fromSecretFile(path: Path, clock: Clock = Clock.systemUTC()): EngineApiJwtTokenProvider {
      val secretHex =
        try {
          path.readText().trim()
        } catch (e: NoSuchFileException) {
          throw IllegalArgumentException("Could not find execution engine JWT secret file: ${path.toAbsolutePath()}", e)
        } catch (e: IOException) {
          throw IllegalArgumentException("Could not read execution engine JWT secret file: ${path.toAbsolutePath()}", e)
        }
      val secret =
        runCatching { secretHex.decodeHex() }
          .getOrElse { throw IllegalArgumentException("JWT secret file $path does not contain a valid hex secret", it) }
      return EngineApiJwtTokenProvider(secret, clock)
        .also { log.info("JWT secret loaded from {}", path.toAbsolutePath()) }
    }
  }
}
