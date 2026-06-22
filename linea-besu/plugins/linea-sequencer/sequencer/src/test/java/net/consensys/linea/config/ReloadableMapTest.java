/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.config;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.util.HashMap;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import org.junit.jupiter.api.Test;

class ReloadableMapTest {

  @Test
  void constructorInitializesWithProvidedMap() {
    final Map<String, Integer> initialMap = Map.of("a", 1, "b", 2, "c", 3);
    final ReloadableMap<String, Integer> reloadableMap = new ReloadableMap<>(initialMap);

    assertThat(reloadableMap).containsExactlyInAnyOrderEntriesOf(initialMap);
  }

  @Test
  void swapReplacesUnderlyingMap() {
    final ReloadableMap<String, Integer> reloadableMap = new ReloadableMap<>(Map.of("original", 1));

    assertThat(reloadableMap).containsExactlyEntriesOf(Map.of("original", 1));

    reloadableMap.swap(Map.of("swapped", 2, "values", 3));

    assertThat(reloadableMap).containsExactlyInAnyOrderEntriesOf(Map.of("swapped", 2, "values", 3));
  }

  @Test
  void getDelegatesToUnderlyingMap() {
    final ReloadableMap<String, Integer> reloadableMap =
        new ReloadableMap<>(Map.of("a", 1, "b", 2, "c", 3));

    assertThat(reloadableMap.get("a")).isEqualTo(1);
    assertThat(reloadableMap.get("b")).isEqualTo(2);
    assertThat(reloadableMap.get("z")).isNull();
  }

  @Test
  void containsKeyDelegatesToUnderlyingMap() {
    final ReloadableMap<String, Integer> reloadableMap =
        new ReloadableMap<>(Map.of("a", 1, "b", 2));

    assertThat(reloadableMap.containsKey("a")).isTrue();
    assertThat(reloadableMap.containsKey("z")).isFalse();
  }

  @Test
  void containsValueDelegatesToUnderlyingMap() {
    final ReloadableMap<String, Integer> reloadableMap =
        new ReloadableMap<>(Map.of("a", 1, "b", 2));

    assertThat(reloadableMap.containsValue(1)).isTrue();
    assertThat(reloadableMap.containsValue(99)).isFalse();
  }

  @Test
  void sizeDelegatesToUnderlyingMap() {
    final ReloadableMap<String, Integer> reloadableMap =
        new ReloadableMap<>(Map.of("a", 1, "b", 2, "c", 3));
    assertThat(reloadableMap.size()).isEqualTo(3);
  }

  @Test
  void isEmptyDelegatesToUnderlyingMap() {
    final ReloadableMap<String, Integer> emptyMap = new ReloadableMap<>(Map.of());
    final ReloadableMap<String, Integer> nonEmptyMap = new ReloadableMap<>(Map.of("a", 1));

    assertThat(emptyMap.isEmpty()).isTrue();
    assertThat(nonEmptyMap.isEmpty()).isFalse();
  }

  @Test
  void keySetReturnsSnapshotOfCurrentMap() {
    final ReloadableMap<String, Integer> reloadableMap =
        new ReloadableMap<>(Map.of("a", 1, "b", 2, "c", 3));

    Set<String> keySet = reloadableMap.keySet();

    // Swap to a different map
    reloadableMap.swap(Map.of("x", 10, "y", 20, "z", 30));

    // keySet should still reflect the original map's keys
    assertThat(keySet).containsExactlyInAnyOrder("a", "b", "c");
  }

  @Test
  void valuesDelegatesToUnderlyingMap() {
    final ReloadableMap<String, Integer> reloadableMap =
        new ReloadableMap<>(Map.of("a", 1, "b", 2));

    assertThat(reloadableMap.values()).containsExactlyInAnyOrder(1, 2);
  }

  @Test
  void entrySetDelegatesToUnderlyingMap() {
    final ReloadableMap<String, Integer> reloadableMap =
        new ReloadableMap<>(Map.of("a", 1, "b", 2));

    Set<Map.Entry<String, Integer>> entrySet = reloadableMap.entrySet();
    assertThat(entrySet).hasSize(2);

    Map<String, Integer> fromEntries = new HashMap<>();
    for (Map.Entry<String, Integer> entry : entrySet) {
      fromEntries.put(entry.getKey(), entry.getValue());
    }
    assertThat(fromEntries).containsExactlyInAnyOrderEntriesOf(Map.of("a", 1, "b", 2));
  }

  @Test
  void equalsReflectsUnderlyingMap() {
    final ReloadableMap<String, Integer> reloadableMap =
        new ReloadableMap<>(Map.of("a", 1, "b", 2));

    assertThat(reloadableMap.equals(Map.of("a", 1, "b", 2))).isTrue();
    assertThat(reloadableMap.equals(Map.of("a", 1))).isFalse();
    assertThat(reloadableMap.equals(reloadableMap)).isTrue();
  }

  @Test
  void hashCodeReflectsUnderlyingMap() {
    final ReloadableMap<String, Integer> reloadableMap =
        new ReloadableMap<>(Map.of("a", 1, "b", 2));

    assertThat(reloadableMap.hashCode()).isEqualTo(Map.of("a", 1, "b", 2).hashCode());
  }

  @Test
  void equalsWorksWithAnotherReloadableMap() {
    final ReloadableMap<String, Integer> map1 = new ReloadableMap<>(Map.of("a", 1));
    final ReloadableMap<String, Integer> map2 = new ReloadableMap<>(Map.of("a", 1));
    final ReloadableMap<String, Integer> map3 = new ReloadableMap<>(Map.of("b", 2));

    assertThat(map1.equals(map2)).isTrue();
    assertThat(map1.equals(map3)).isFalse();
  }

  @Test
  void equalsUpdatesAfterSwap() {
    final ReloadableMap<String, Integer> reloadableMap =
        new ReloadableMap<>(Map.of("a", 1, "b", 2));

    assertThat(reloadableMap.equals(Map.of("a", 1, "b", 2))).isTrue();

    reloadableMap.swap(Map.of("x", 10, "y", 20));

    assertThat(reloadableMap.equals(Map.of("a", 1, "b", 2))).isFalse();
    assertThat(reloadableMap.equals(Map.of("x", 10, "y", 20))).isTrue();
  }

  @Test
  void putThrowsUnsupportedOperationException() {
    final ReloadableMap<String, Integer> reloadableMap = new ReloadableMap<>(Map.of());

    assertThatThrownBy(() -> reloadableMap.put("b", 2))
        .isInstanceOf(UnsupportedOperationException.class)
        .hasMessageContaining("immutable");
  }

  @Test
  void removeThrowsUnsupportedOperationException() {
    final ReloadableMap<String, Integer> reloadableMap = new ReloadableMap<>(Map.of());

    assertThatThrownBy(() -> reloadableMap.remove("a"))
        .isInstanceOf(UnsupportedOperationException.class)
        .hasMessageContaining("immutable");
  }

  @Test
  void putAllThrowsUnsupportedOperationException() {
    final ReloadableMap<String, Integer> reloadableMap = new ReloadableMap<>(Map.of("a", 1));

    assertThatThrownBy(() -> reloadableMap.putAll(Map.of("b", 2, "c", 3)))
        .isInstanceOf(UnsupportedOperationException.class)
        .hasMessageContaining("immutable");
  }

  @Test
  void clearThrowsUnsupportedOperationException() {
    final ReloadableMap<String, Integer> reloadableMap = new ReloadableMap<>(Map.of("a", 1));

    assertThatThrownBy(reloadableMap::clear)
        .isInstanceOf(UnsupportedOperationException.class)
        .hasMessageContaining("immutable");
  }

  @Test
  void concurrentIteratorsAlwaysSeeConsistentSnapshot() throws InterruptedException {
    // All swapped maps have exactly 3 entries — an entrySet iterator snapshot should never see any
    // other count
    final ReloadableMap<Integer, String> reloadableMap =
        new ReloadableMap<>(Map.of(1, "a", 2, "b", 3, "c"));

    final int threadCount = 10;
    final int operationsPerThread = 1000;
    final ExecutorService executor = Executors.newFixedThreadPool(threadCount);
    final CountDownLatch latch = new CountDownLatch(threadCount);
    final AtomicInteger errorCount = new AtomicInteger(0);

    for (int i = 0; i < threadCount; i++) {
      final int threadId = i;
      executor.submit(
          () -> {
            try {
              for (int j = 0; j < operationsPerThread; j++) {
                if (threadId == 0) {
                  // Writer thread: swap to a new map of size 3
                  reloadableMap.swap(
                      Map.of(
                          j * 3,
                          "v" + j * 3,
                          j * 3 + 1,
                          "v" + (j * 3 + 1),
                          j * 3 + 2,
                          "v" + (j * 3 + 2)));
                } else {
                  // Reader threads: entrySet iterator must be a consistent snapshot of exactly 3
                  // entries
                  int count = 0;
                  for (var it = reloadableMap.entrySet().iterator(); it.hasNext(); it.next()) {
                    count++;
                  }
                  if (count != 3) {
                    errorCount.incrementAndGet();
                  }
                }
              }
            } catch (Exception e) {
              errorCount.incrementAndGet();
            } finally {
              latch.countDown();
            }
          });
    }

    assertThat(latch.await(10, TimeUnit.SECONDS)).isTrue();
    executor.shutdown();

    assertThat(errorCount.get()).isEqualTo(0);
  }

  @Test
  void concurrentReadsAndSwapsAlwaysSeeConsistentSnapshot() throws InterruptedException {
    // All swapped maps have exactly 3 entries — a reader should never see any other size
    final ReloadableMap<Integer, String> reloadableMap =
        new ReloadableMap<>(Map.of(1, "a", 2, "b", 3, "c"));

    final int threadCount = 10;
    final int operationsPerThread = 1000;
    final ExecutorService executor = Executors.newFixedThreadPool(threadCount);
    final CountDownLatch latch = new CountDownLatch(threadCount);
    final AtomicInteger errorCount = new AtomicInteger(0);

    for (int i = 0; i < threadCount; i++) {
      final int threadId = i;
      executor.submit(
          () -> {
            try {
              for (int j = 0; j < operationsPerThread; j++) {
                if (threadId == 0) {
                  // Writer thread: swap to a new map of size 3
                  reloadableMap.swap(
                      Map.of(
                          j * 3,
                          "v" + j * 3,
                          j * 3 + 1,
                          "v" + (j * 3 + 1),
                          j * 3 + 2,
                          "v" + (j * 3 + 2)));
                } else {
                  // Reader threads: size must always be 3
                  int size = reloadableMap.size();
                  if (size != 3) {
                    errorCount.incrementAndGet();
                  }
                  // get should never throw
                  reloadableMap.get(j);
                  reloadableMap.containsKey(j);
                }
              }
            } catch (Exception e) {
              errorCount.incrementAndGet();
            } finally {
              latch.countDown();
            }
          });
    }

    assertThat(latch.await(10, TimeUnit.SECONDS)).isTrue();
    executor.shutdown();

    assertThat(errorCount.get()).isEqualTo(0);
  }
}
