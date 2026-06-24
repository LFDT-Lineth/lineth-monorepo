/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package testutils

import org.apache.logging.log4j.ThreadContext
import org.junit.jupiter.api.extension.AfterAllCallback
import org.junit.jupiter.api.extension.AfterEachCallback
import org.junit.jupiter.api.extension.BeforeAllCallback
import org.junit.jupiter.api.extension.BeforeEachCallback
import org.junit.jupiter.api.extension.ExtensionContext

/**
 * Routes every integration test's logs to its own file so they stay readable when tests run
 * concurrently. The RoutingAppender in `log4j2-test.xml` resolves the destination per log event:
 *
 *  - `testId` ([ThreadContext]) -> per-method file. Set on the test's worker thread in
 *    [beforeEach]; because `log4j2.isThreadContextMapInheritable=true` for the integrationTest JVM,
 *    Besu/Maru worker threads the test spawns inherit it and log into the same file.
 *  - `maru.test.class` (system property) -> per-class fallback file. Set once per fork in
 *    [beforeAll]. It is JVM-global (not thread-local), so it captures logs from JVM-global pools
 *    like `ForkJoinPool.commonPool`, whose threads are created outside any test's [ThreadContext]
 *    scope and so never inherit a `testId`. This is reliable because the build runs one test class
 *    per JVM (`forkEvery = 1`) with methods executing sequentially (`same_thread`).
 *
 * Auto-registered for the integrationTest source set via
 * `META-INF/services/org.junit.jupiter.api.extension.Extension` together with
 * `junit.jupiter.extensions.autodetection.enabled=true` (see junit-platform.properties), so no test
 * class needs to opt in.
 */
class PerTestLoggingExtension :
  BeforeAllCallback,
  AfterAllCallback,
  BeforeEachCallback,
  AfterEachCallback {
  override fun beforeAll(context: ExtensionContext) {
    context.testClass.ifPresent { System.setProperty(TEST_CLASS_KEY, sanitize(it.simpleName)) }
  }

  override fun afterAll(context: ExtensionContext) {
    System.clearProperty(TEST_CLASS_KEY)
  }

  override fun beforeEach(context: ExtensionContext) {
    ThreadContext.put(TEST_ID_KEY, testIdOf(context))
  }

  override fun afterEach(context: ExtensionContext) {
    ThreadContext.remove(TEST_ID_KEY)
  }

  private fun testIdOf(context: ExtensionContext): String {
    val className = context.testClass.map { it.simpleName }.orElse("UnknownClass")
    return sanitize("$className.${context.displayName}")
  }

  private fun sanitize(name: String): String = name.replace(UNSAFE_FILENAME_CHARS, "_").trim('_')

  companion object {
    private const val TEST_ID_KEY = "testId"
    private const val TEST_CLASS_KEY = "maru.test.class"
    private val UNSAFE_FILENAME_CHARS = Regex("[^A-Za-z0-9._-]+")
  }
}
