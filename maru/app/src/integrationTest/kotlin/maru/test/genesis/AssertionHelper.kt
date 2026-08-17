/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.test.genesis

import com.fasterxml.jackson.databind.JsonNode
import org.assertj.core.api.Assertions

fun assertIsNumberWithValue(
  node: JsonNode,
  expectedValue: Long,
) {
  Assertions.assertThat(node.isNumber).isTrue
  Assertions.assertThat(node.asLong()).isEqualTo(expectedValue)
}

fun assertIsNumberWithValue(
  node: JsonNode,
  expectedValue: ULong,
) = assertIsNumberWithValue(node, expectedValue.toLong())

fun assertIsBooleanWithValue(
  node: JsonNode,
  expectedValue: Boolean,
) {
  Assertions.assertThat(node.isBoolean).isTrue
  Assertions.assertThat(node.asBoolean()).isEqualTo(expectedValue)
}
