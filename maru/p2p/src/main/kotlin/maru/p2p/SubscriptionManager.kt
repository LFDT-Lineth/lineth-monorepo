/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p

import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicInteger
import java.util.function.Supplier

class SubscriptionManager<E> {
  private val log = LogManager.getLogger(this.javaClass)
  private val nextSubscriptionId = AtomicInteger()
  private val subscriptions: MutableMap<Int, (E) -> SafeFuture<ValidationResult>> = mutableMapOf()

  fun hasSubscriptions(): Boolean = subscriptions.isNotEmpty()

  @Synchronized
  fun subscribe(subscriber: (E) -> SafeFuture<ValidationResult>): Int {
    val subscriptionId = nextSubscriptionId.get()
    subscriptions[subscriptionId] = subscriber
    nextSubscriptionId.incrementAndGet()
    return subscriptionId
  }

  @Synchronized
  fun unsubscribe(subscriptionId: Int) {
    subscriptions.remove(subscriptionId)
  }

  fun handleEvent(event: E): SafeFuture<ValidationResult> {
    val handlerFutures =
      subscriptions.map { (subscriptionId, handler) ->
        try {
          handler(event)
        } catch (th: Throwable) {
          log.debug(
            Supplier<String> { "Error from subscription=$subscriptionId while handling event=$event!" },
            th,
          )
          SafeFuture.completedFuture<ValidationResult>(
            ValidationResult.Companion.Invalid(
              "Exception during event handling",
              th,
            ),
          )
        }
      }
    return if (subscriptions.isNotEmpty()) {
      SafeFuture.collectAll(handlerFutures.stream()).thenApply {
        it.reduce { acc: ValidationResult, next: ValidationResult ->
          when {
            acc is ValidationResult.Companion.Invalid -> acc
            next is ValidationResult.Companion.Invalid -> next
            acc is ValidationResult.Companion.Ignore -> acc
            next is ValidationResult.Companion.Ignore -> next
            else -> acc
          }
        }
      }
    } else {
      SafeFuture.completedFuture(
        ValidationResult.Companion.Ignore(
          "No subscription to imply message validity",
        ),
      )
    }
  }
}
