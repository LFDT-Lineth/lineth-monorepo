/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.extensions

import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.metrics.DynamicTagTimer
import net.consensys.linea.metrics.Timer
import tech.pegasys.teku.infrastructure.async.SafeFuture

fun <T> Timer.captureTimeSafeFuture(safeFuture: SafeFuture<T>): SafeFuture<T> =
  this.captureTime(safeFuture.toCompletableFuture()).toSafeFuture()

fun <T> DynamicTagTimer<T>.captureTimeSafeFuture(safeFuture: SafeFuture<T>): SafeFuture<T> =
  this.captureTime(safeFuture.toCompletableFuture()).toSafeFuture()
