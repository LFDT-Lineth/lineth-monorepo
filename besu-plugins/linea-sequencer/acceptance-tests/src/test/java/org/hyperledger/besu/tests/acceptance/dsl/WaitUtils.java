/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package org.hyperledger.besu.tests.acceptance.dsl;

import java.util.concurrent.TimeUnit;
import org.awaitility.Awaitility;
import org.awaitility.core.ThrowingRunnable;

/** Contains functionality for timeouts. */
public class WaitUtils {
  public static void waitFor(final ThrowingRunnable condition) {
    waitFor(30, condition);
  }

  public static void waitFor(final int timeout, final ThrowingRunnable condition) {
    Awaitility.await()
        .pollInterval(5, TimeUnit.SECONDS)
        .ignoreExceptions()
        .atMost(timeout, TimeUnit.SECONDS)
        .untilAsserted(condition);
  }
}
