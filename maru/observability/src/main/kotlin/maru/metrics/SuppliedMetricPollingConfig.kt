/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.metrics

import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

data class SuppliedMetricPollingConfig(
  val initialDelay: Duration = 1.seconds,
  val pollingInterval: Duration = 1.seconds,
)
