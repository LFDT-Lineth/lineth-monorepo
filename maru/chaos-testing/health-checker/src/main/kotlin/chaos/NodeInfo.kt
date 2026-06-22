/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package chaos

data class NodeInfo<T>(
  val label: String,
  val value: T,
) {
  fun <R> map(fn: (T) -> R): NodeInfo<R> = NodeInfo(label, fn(value))

  fun <R> map(newValue: R): NodeInfo<R> = NodeInfo(label, newValue)
}
