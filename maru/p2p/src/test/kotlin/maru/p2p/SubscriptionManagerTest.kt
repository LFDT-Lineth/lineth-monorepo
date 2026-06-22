/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import tech.pegasys.teku.infrastructure.async.SafeFuture

class SubscriptionManagerTest {
  private lateinit var manager: SubscriptionManager<String>

  @BeforeEach
  fun setUp() {
    manager = SubscriptionManager()
  }

  @Test
  fun `hasSubscriptions returns false when empty and true after subscribe`() {
    assertThat(manager.hasSubscriptions()).isFalse()

    manager.subscribe { SafeFuture.completedFuture(ValidationResult.Companion.Valid) }
    assertThat(manager.hasSubscriptions()).isTrue()
  }

  @Test
  fun `subscribe returns unique ids and can unsubscribe`() {
    val id1 = manager.subscribe { SafeFuture.completedFuture(ValidationResult.Companion.Valid) }
    val id2 = manager.subscribe { SafeFuture.completedFuture(ValidationResult.Companion.Valid) }
    assertThat(id2).isNotEqualTo(id1)

    manager.unsubscribe(id1)
    assertThat(manager.hasSubscriptions()).isTrue()
    manager.unsubscribe(id2)
    assertThat(manager.hasSubscriptions()).isFalse()
  }

  @Test
  fun `handleEvent calls all subscribers and aggregates ValidationResult`() {
    val results = mutableListOf<String>()
    manager.subscribe {
      results.add("first")
      SafeFuture.completedFuture(ValidationResult.Companion.Valid)
    }
    manager.subscribe {
      results.add("second")
      SafeFuture.completedFuture(ValidationResult.Companion.Ignore("meh"))
    }
    val future = manager.handleEvent("event")
    val result = future.get()
    assertThat(results).containsExactly("first", "second")
    assertThat(result).isInstanceOf(ValidationResult.Companion.Ignore::class.java)
  }

  @Test
  fun `handleEvent returns Failed if any subscriber fails`() {
    manager.subscribe {
      SafeFuture.completedFuture(ValidationResult.Companion.Valid)
    }
    manager.subscribe {
      SafeFuture.completedFuture(ValidationResult.Companion.Invalid("fail"))
    }
    val result = manager.handleEvent("event").get()
    assertThat(result).isInstanceOf(ValidationResult.Companion.Invalid::class.java)
  }

  @Test
  fun `handleEvent returns KindaFine if no subscriptions`() {
    val result = manager.handleEvent("event").get()
    assertThat(result).isInstanceOf(ValidationResult.Companion.Ignore::class.java)
  }

  @Test
  fun `handleEvent handles subscriber exceptions gracefully`() {
    manager.subscribe {
      throw RuntimeException("boom")
    }
    val result = manager.handleEvent("event").get()
    assertThat(result).isInstanceOf(ValidationResult.Companion.Invalid::class.java)
  }

  @Test
  fun `handleEvent aggregates with Failed over KindaFine and Successful`() {
    manager.subscribe {
      SafeFuture.completedFuture(ValidationResult.Companion.Valid)
    }
    manager.subscribe {
      SafeFuture.completedFuture(ValidationResult.Companion.Ignore("meh"))
    }
    manager.subscribe {
      SafeFuture.completedFuture(ValidationResult.Companion.Invalid("fail"))
    }
    val result = manager.handleEvent("event").get()
    assertThat(result).isInstanceOf(ValidationResult.Companion.Invalid::class.java)
  }

  @Test
  fun `handleEvent aggregates with KindaFine over Successful`() {
    manager.subscribe {
      SafeFuture.completedFuture(ValidationResult.Companion.Valid)
    }
    manager.subscribe {
      SafeFuture.completedFuture(ValidationResult.Companion.Ignore("meh"))
    }
    val result = manager.handleEvent("event").get()
    assertThat(result).isInstanceOf(ValidationResult.Companion.Ignore::class.java)
  }

  @Test
  fun `only subscribed handler is called after unsubscription`() {
    var called1 = false
    var called2 = false
    val id1 =
      manager.subscribe {
        called1 = true
        SafeFuture.completedFuture(ValidationResult.Companion.Valid)
      }
    manager.subscribe {
      called2 = true
      SafeFuture.completedFuture(ValidationResult.Companion.Valid)
    }
    manager.unsubscribe(id1)
    manager.handleEvent("event").get()
    assertThat(called1).isFalse()
    assertThat(called2).isTrue()
  }
}
