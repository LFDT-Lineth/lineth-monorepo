/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.subscription

import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CopyOnWriteArrayList

open class SubscriberRootClass(
  val subscriptionManager: SubscriptionManager<String>,
  val notifications: MutableList<String> = CopyOnWriteArrayList(),
  val subscriberLabel: String = "SubscriberRootClass",
) : Observable<String> by subscriptionManager {
  fun syncHandler(data: String) {
    notifications.add("$subscriberLabel:syncHandler called with: $data")
  }

  fun asyncHandler(data: String): SafeFuture<String> {
    notifications.add("$subscriberLabel:asyncHandler called with: $data")
    return SafeFuture.completedFuture("hello")
  }

  /**
   * Subscribes to the subscription manager and returns a list of subscription IDs.
   * The IDs can be used to unsubscribe later if needed.
   */
  fun subscribe(): List<String> =
    listOf(
      subscriptionManager.addSyncSubscriber(this::syncHandler),
      subscriptionManager.addAsyncSubscriber(this::asyncHandler),
    )
}
