/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.subscription

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CopyOnWriteArrayList

class InOrderFanoutSubscriptionManagerTest {
  private lateinit var subscriptionManager: InOrderFanoutSubscriptionManager<String>

  @BeforeEach
  fun setUp() {
    subscriptionManager = InOrderFanoutSubscriptionManager()
  }

  @Test
  fun `should throw when handlerId is already assigned`() {
    subscriptionManager.addSyncSubscriber("handler1", { data -> "handler1 called with: $data" })
    assertThrows<IllegalArgumentException> {
      subscriptionManager.addSyncSubscriber("handler1", { data -> "handler1 called with: $data" })
    }
    assertThrows<IllegalArgumentException> {
      subscriptionManager.addAsyncSubscriber(
        "handler1",
        { data -> SafeFuture.completedFuture("handler1 called with: $data") },
      )
    }
  }

  @Test
  fun `should throw when handler is already subscribed even with different id`() {
    val syncHandler = { data: String -> "sync handler called with: $data" }
    val aSyncHandler = { data: String -> SafeFuture.completedFuture("async handler called with $data") }

    subscriptionManager.addSyncSubscriber("handler1", syncHandler)
    assertThrows<IllegalArgumentException> {
      subscriptionManager.addSyncSubscriber("handler2", syncHandler)
    }

    subscriptionManager.addAsyncSubscriber("async handler1", aSyncHandler)
    assertThrows<IllegalArgumentException> {
      subscriptionManager.addSyncSubscriber("async handler2", aSyncHandler)
    }
  }

  class SubscriberInnerClass(
    subscriptionManager: SubscriptionManager<String>,
    notifications: MutableList<String> = CopyOnWriteArrayList(),
    subscriberLabel: String = "SubscriberInnerClass",
  ) : SubscriberRootClass(
    subscriptionManager = subscriptionManager,
    notifications = notifications,
    subscriberLabel = subscriberLabel,
  )

  @Test
  fun `should assign id when not provided and remove them`() {
    val notifications = CopyOnWriteArrayList<String>()
    val subscribersIds = mutableListOf<String>()

    SubscriberRootClass(subscriptionManager, notifications, "subscriberRootClass1")
      .also {
        val ids = it.subscribe()
        subscribersIds.addAll(ids)
        assertThat(ids[0]).startsWith("maru.subscription.SubscriberRootClass.syncHandler")
        assertThat(ids[1]).startsWith("maru.subscription.SubscriberRootClass.asyncHandler")
      }

    SubscriberRootClass(subscriptionManager, notifications, "subscriberRootClass2")
      .also {
        val id1 = subscriptionManager.addSyncSubscriber(it::syncHandler).also { subscribersIds.add(it) }
        val id2 = subscriptionManager.addAsyncSubscriber(it::asyncHandler).also { subscribersIds.add(it) }

        assertThat(id1).startsWith("maru.subscription.SubscriberRootClass.syncHandler")
        assertThat(id2).startsWith("maru.subscription.SubscriberRootClass.asyncHandler")
      }

    SubscriberInnerClass(subscriptionManager, notifications, "subscriberRootClass2")
      .also {
        val id1 = subscriptionManager.addSyncSubscriber(it::syncHandler).also { subscribersIds.add(it) }
        val id2 = subscriptionManager.addAsyncSubscriber(it::asyncHandler).also { subscribersIds.add(it) }
        assertThat(
          id1.split("@").first(),
        ).isEqualTo("maru.subscription.InOrderFanoutSubscriptionManagerTest\$SubscriberInnerClass.syncHandler")
        assertThat(
          id2.split("@").first(),
        ).isEqualTo("maru.subscription.InOrderFanoutSubscriptionManagerTest\$SubscriberInnerClass.asyncHandler")
      }

    val lambdaSubscriberIdPrefix =
      "maru.subscription.InOrderFanoutSubscriptionManagerTest.should assign id when not provided " +
        "and remove them(InOrderFanoutSubscriptionManagerTest.kt"
    SubscriberRootClass(subscriptionManager, notifications, "subscriberRootClass3")
      .also { subscriber ->
        val id1 =
          subscriptionManager
            .addSyncSubscriber(syncHandler = { data -> subscriber.syncHandler(data) })
            .also { subscribersIds.add(it) }
        val id2 =
          subscriptionManager
            .addAsyncSubscriber(asyncHandler = { data -> subscriber.asyncHandler(data) })
            .also { subscribersIds.add(it) }

        assertThat(id1).startsWith(lambdaSubscriberIdPrefix)
        assertThat(id2).startsWith(lambdaSubscriberIdPrefix)
      }

    val lambdaHandler = { data: String ->
      notifications.add("lambda1 sync handler called with: $data")
    }
    subscriptionManager.addSyncSubscriber(lambdaHandler).also { id ->
      assertThat(id).startsWith(lambdaSubscriberIdPrefix)
      subscribersIds.add(id)
    }

    class MyObservable(
      val subscriptionManager: SubscriptionManager<String>,
    ) : Observable<String> by subscriptionManager

    // using interface implementation delegation
    MyObservable(subscriptionManager)
      .addSyncSubscriber({ data -> notifications.add("lambda2 sync handler called with: $data") })
      .also { subscribersIds.add(it) }

    val lambdaSubscriberIdPrefix2 =
      "maru.subscription.InOrderFanoutSubscriptionManagerTest." +
        "should_assign_id_when_not_provided_and_remove_them\$addSubscriber(InOrderFanoutSubscriptionManagerTest.kt"

    fun addSubscriber(handlerName: String) {
      val subsId =
        subscriptionManager.addSyncSubscriber({ data ->
          notifications.add("$handlerName called with: $data")
        })
      subscribersIds.add(subsId)
      assertThat(subsId).startsWith(lambdaSubscriberIdPrefix2)
    }

    addSubscriber("inline lambada1")
    addSubscriber("inline lambada2")

    /**
     * Examples of subscriberIds if we log them:
     subscriberRootClass1
     "maru.subscription.SubscriberRootClass.syncHandler1808826734",
     "maru.subscription.SubscriberRootClass.asyncHandler1808826734",
     subscriberRootClass2
     "maru.subscription.SubscriberRootClass.syncHandler133845838",
     "maru.subscription.SubscriberRootClass.asyncHandler133845838",
     lambdas:
     "maru.subscription.InOrderFanoutSubscriptionManagerTest.should assign id when not provided and remove them
     by reference - root class(InOrderFanoutSubscriptionManagerTest.kt:88)",
     "maru.subscription.InOrderFanoutSubscriptionManagerTest.should assign id when not provided and remove them
     by reference - root class(InOrderFanoutSubscriptionManagerTest.kt:89)",
     "maru.subscription.InOrderFanoutSubscriptionManagerTest.should assign id when not provided and remove them
     by reference - root class(InOrderFanoutSubscriptionManagerTest.kt:92)"
     "maru.subscription.InOrderFanoutSubscriptionManagerTest.should assign id when not provided and remove them
     by reference - root class(InOrderFanoutSubscriptionManagerTest.kt:108)"
     */

    subscriptionManager.notifySubscribers("d1")
    subscribersIds.forEach { subscriptionManager.removeSubscriber(it) }

    subscriptionManager.notifySubscribers("d2")
    assertThat(notifications.filter { it.contains("d2") }).isEmpty()
  }

  @Test
  fun `should notify in subscription order`() {
    val notifications = CopyOnWriteArrayList<String>()
    val numberOfHandlers = 40
    (1..numberOfHandlers).forEach { i ->
      if (i % 2 == 0) {
        subscriptionManager.addSyncSubscriber(
          "handler$i",
          { data -> notifications.add("handler$i called with: $data") },
        )
      } else {
        subscriptionManager.addAsyncSubscriber("handler$i", { data ->
          notifications.add("handler$i called with: $data")
          SafeFuture.runAsync({
            Thread.sleep(5) // Simulate async processing delay
            "handler2 called with: $data"
          })
        })
      }
    }

    subscriptionManager.notifySubscribers("d1")
    subscriptionManager.notifySubscribersAsync("d2")
    subscriptionManager.notifySubscribers("d3")
    subscriptionManager.notifySubscribersAsync("d4")

    val expectedNotifications1 = (1..numberOfHandlers).map { i -> "handler$i called with: d1" }
    val expectedNotifications2 = (1..numberOfHandlers).map { i -> "handler$i called with: d2" }
    val expectedNotifications3 = (1..numberOfHandlers).map { i -> "handler$i called with: d3" }
    val expectedNotifications4 = (1..numberOfHandlers).map { i -> "handler$i called with: d4" }

    assertThat(notifications).isEqualTo(
      expectedNotifications1 + expectedNotifications2 + expectedNotifications3 + expectedNotifications4,
    )
  }

  @Test
  fun `notifyAsync should notify subscribers without blocking calling thread`() {
    val notifications = CopyOnWriteArrayList<String>()
    subscriptionManager.addSyncSubscriber("handler1", { data -> notifications.add("handler1 called with: $data") })
    val futures = CopyOnWriteArrayList<SafeFuture<String>>()
    subscriptionManager.addAsyncSubscriber("handler2", { data ->
      val futureResult = SafeFuture<String>()
      futureResult.thenPeek { notifications.add("async handler2 called with: $data") }
      futures.add(futureResult)
      futureResult
    })

    subscriptionManager.notifySubscribersAsync("d1")
    subscriptionManager.notifySubscribersAsync("d2")
    subscriptionManager.notifySubscribersAsync("d3")
    // only handler1 is synchronous, so it should be called immediately
    // handler2 is async, so we need to wait for it
    assertThat(notifications).isEqualTo(
      listOf(
        "handler1 called with: d1",
        "handler1 called with: d2",
        "handler1 called with: d3",
      ),
    )
    futures.forEach { it.complete("handler2 completed") }

    assertThat(notifications).isEqualTo(
      listOf(
        "handler1 called with: d1",
        "handler1 called with: d2",
        "handler1 called with: d3",
        "async handler2 called with: d1",
        "async handler2 called with: d2",
        "async handler2 called with: d3",
      ),
    )
  }

  @Test
  fun `should be resilient to exceptions in subscribers`() {
    val notifications = CopyOnWriteArrayList<String>()
    subscriptionManager.addSyncSubscriber("handler1", { data ->
      notifications.add("handler1 called with: $data")
      throw RuntimeException("error in handler1")
    })
    subscriptionManager.addAsyncSubscriber("handler2", { data ->
      notifications.add("handler2 called with: $data")
      throw RuntimeException("error in handler2")
    })
    subscriptionManager.addAsyncSubscriber("handler3", { data ->
      notifications.add("handler3 called with: $data")
      SafeFuture.runAsync({
        Thread.sleep(50) // Simulate async processing delay
        throw RuntimeException("error in handler3")
      })
    })
    subscriptionManager.addSyncSubscriber("handler4", { data ->
      notifications.add("handler4 called with: $data")
    })
    subscriptionManager.addAsyncSubscriber("handler5", { data ->
      notifications.add("handler5 called with: $data")
      SafeFuture.completedFuture<String>("handler5 completed with: $data")
    })

    subscriptionManager.notifySubscribers("d1")
    subscriptionManager.notifySubscribersAsync("d2")

    assertThat(notifications).isEqualTo(
      listOf(
        "handler1 called with: d1",
        "handler2 called with: d1",
        "handler3 called with: d1",
        "handler4 called with: d1",
        "handler5 called with: d1",
        "handler1 called with: d2",
        "handler2 called with: d2",
        "handler3 called with: d2",
        "handler4 called with: d2",
        "handler5 called with: d2",
      ),
    )
  }

  @Test
  fun `should not notify removed subscribers`() {
    val notifications = CopyOnWriteArrayList<String>()

    subscriptionManager.addSyncSubscriber("handler1", { data ->
      notifications.add("handler1 called with: $data")
    })
    subscriptionManager.notifySubscribers("d1")
    assertThat(notifications).isEqualTo(listOf("handler1 called with: d1"))

    subscriptionManager.removeSubscriber("handler1")

    subscriptionManager.notifySubscribers("d2")
    assertThat(notifications).isEqualTo(listOf("handler1 called with: d1"))
  }

  @Test
  fun `should remove subscriber by handler reference`() {
    val notifications = CopyOnWriteArrayList<String>()
    val syncHandler1 = { data: String -> notifications.add("sync handler1 called with: $data") }
    val asyncHandler2 = { data: String ->
      notifications.add("sync handler2 called with: $data")
      SafeFuture.completedFuture("async handler2 completed with: $data")
    }
    subscriptionManager.addSyncSubscriber(subscriberId = "sync handler1", syncHandler = syncHandler1)
    subscriptionManager.addSyncSubscriber(syncHandler = asyncHandler2)
    subscriptionManager.notifySubscribers("d1")
    assertThat(notifications).hasSize(2)

    subscriptionManager.removeSubscriber(syncHandler1)
    subscriptionManager.removeSubscriber(asyncHandler2)
    subscriptionManager.notifySubscribers("d2")

    assertThat(notifications).hasSize(2)
  }

  @Test
  fun `should remove subscribers by observer object reference`() {
    val notifications = CopyOnWriteArrayList<String>()
    val observer =
      SubscriberRootClass(
        subscriptionManager = subscriptionManager,
        notifications = notifications,
        subscriberLabel = "Observer",
      ).also { it.subscribe() }

    subscriptionManager.notifySubscribers("d1")
    assertThat(notifications).hasSize(2)
    subscriptionManager.removeSubscriber(observer)
    subscriptionManager.notifySubscribers("d2")
    assertThat(notifications).hasSize(2)
  }

  @Test
  fun `should handle empty subscription list gracefully`() {
    subscriptionManager.notifySubscribers("d1")
    subscriptionManager.notifySubscribersAsync("d2")
  }
}
