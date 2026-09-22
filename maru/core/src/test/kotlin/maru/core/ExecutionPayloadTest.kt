/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.core

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class ExecutionPayloadTest {
  private val payload = GENESIS_EXECUTION_PAYLOAD.copy(
    feeRecipient = ByteArray(20) { 1 },
    transactions = listOf(byteArrayOf(1, 2), byteArrayOf(3)),
  )

  @Test
  fun `different fee recipients are not equal`() {
    assertThat(payload).isNotEqualTo(payload.copy(feeRecipient = ByteArray(20) { 2 }))
  }

  @Test
  fun `transaction prefixes are not equal to the full list`() {
    for (transactions in listOf(emptyList(), payload.transactions.take(1))) {
      val prefix = payload.copy(transactions = transactions)
      assertThat(payload).isNotEqualTo(prefix)
      assertThat(prefix).isNotEqualTo(payload)
    }
  }

  @Test
  fun `transaction content and order affect equality`() {
    assertThat(payload).isNotEqualTo(payload.copy(transactions = listOf(byteArrayOf(1, 2), byteArrayOf(4))))
    assertThat(payload).isNotEqualTo(payload.copy(transactions = payload.transactions.reversed()))
  }

  @Test
  fun `equal array contents have equal hashes and work as hash set keys`() {
    val copy = payload.copy(
      feeRecipient = payload.feeRecipient.copyOf(),
      transactions = payload.transactions.map { it.copyOf() },
    )
    assertThat(copy).isEqualTo(payload)
    assertThat(copy.hashCode()).isEqualTo(payload.hashCode())
    assertThat(hashSetOf(payload).contains(copy)).isTrue()
  }
}
