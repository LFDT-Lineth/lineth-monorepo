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

import java.util.HashSet;
import java.util.Iterator;
import java.util.Set;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import org.junit.jupiter.api.Test;

class ReloadableSetTest {

  @Test
  void constructorInitializesWithProvidedSet() {
    final ReloadableSet<String> reloadableSet = new ReloadableSet<>(Set.of("a", "b", "c"));

    assertThat(reloadableSet).containsExactlyInAnyOrder("a", "b", "c");
  }

  @Test
  void swapReplacesUnderlyingSet() {
    final ReloadableSet<String> reloadableSet = new ReloadableSet<>(Set.of("original"));

    assertThat(reloadableSet).containsExactly("original");

    reloadableSet.swap(Set.of("swapped", "values"));

    assertThat(reloadableSet).containsExactlyInAnyOrder("swapped", "values");
  }

  @Test
  void containsDelegatesToUnderlyingSet() {
    final ReloadableSet<String> reloadableSet = new ReloadableSet<>(Set.of("a", "b", "c"));

    assertThat(reloadableSet.contains("a")).isTrue();
    assertThat(reloadableSet.contains("b")).isTrue();
    assertThat(reloadableSet.contains("z")).isFalse();
  }

  @Test
  void sizeDelegatesToUnderlyingSet() {
    final ReloadableSet<String> reloadableSet = new ReloadableSet<>(Set.of("a", "b", "c"));
    assertThat(reloadableSet.size()).isEqualTo(3);
  }

  @Test
  void isEmptyDelegatesToUnderlyingSet() {
    final ReloadableSet<String> emptySet = new ReloadableSet<>(Set.of());
    final ReloadableSet<String> nonEmptySet = new ReloadableSet<>(Set.of("a"));

    assertThat(emptySet.isEmpty()).isTrue();
    assertThat(nonEmptySet.isEmpty()).isFalse();
  }

  @Test
  void iteratorReturnsSnapshotOfCurrentSet() {
    final ReloadableSet<String> reloadableSet = new ReloadableSet<>(Set.of("a", "b", "c"));

    Iterator<String> iterator = reloadableSet.iterator();

    // Swap to a different set
    reloadableSet.swap(Set.of("x", "y", "z"));

    // Iterator should still iterate over the original set
    Set<String> iteratedValues = new HashSet<>();
    while (iterator.hasNext()) {
      iteratedValues.add(iterator.next());
    }

    assertThat(iteratedValues).containsExactlyInAnyOrder("a", "b", "c");
  }

  @Test
  void toArrayDelegatesToUnderlyingSet() {
    final ReloadableSet<String> reloadableSet = new ReloadableSet<>(Set.of("a", "b"));

    Object[] array = reloadableSet.toArray();
    assertThat(array).containsExactlyInAnyOrder("a", "b");

    String[] typedArray = reloadableSet.toArray(new String[0]);
    assertThat(typedArray).containsExactlyInAnyOrder("a", "b");
  }

  @Test
  void containsAllDelegatesToUnderlyingSet() {
    final ReloadableSet<String> reloadableSet = new ReloadableSet<>(Set.of("a", "b", "c"));

    assertThat(reloadableSet.containsAll(Set.of("a", "b"))).isTrue();
    assertThat(reloadableSet.containsAll(Set.of("a", "z"))).isFalse();
  }

  @Test
  void equalsReflectsUnderlyingSet() {
    final ReloadableSet<String> reloadableSet = new ReloadableSet<>(Set.of("a", "b", "c"));

    assertThat(reloadableSet.equals(Set.of("a", "b", "c"))).isTrue();
    assertThat(reloadableSet.equals(Set.of("a", "b"))).isFalse();
    assertThat(reloadableSet.equals(reloadableSet)).isTrue();
  }

  @Test
  void hashCodeReflectsUnderlyingSet() {
    final ReloadableSet<String> reloadableSet = new ReloadableSet<>(Set.of("a", "b", "c"));

    assertThat(reloadableSet.hashCode()).isEqualTo(Set.of("a", "b", "c").hashCode());
  }

  @Test
  void equalsWorksWithAnotherReloadableSet() {
    final ReloadableSet<String> set1 = new ReloadableSet<>(Set.of("a", "b"));
    final ReloadableSet<String> set2 = new ReloadableSet<>(Set.of("a", "b"));
    final ReloadableSet<String> set3 = new ReloadableSet<>(Set.of("c"));

    assertThat(set1.equals(set2)).isTrue();
    assertThat(set1.equals(set3)).isFalse();
  }

  @Test
  void equalsUpdatesAfterSwap() {
    final ReloadableSet<String> reloadableSet = new ReloadableSet<>(Set.of("a", "b"));

    assertThat(reloadableSet.equals(Set.of("a", "b"))).isTrue();

    reloadableSet.swap(Set.of("x", "y"));

    assertThat(reloadableSet.equals(Set.of("a", "b"))).isFalse();
    assertThat(reloadableSet.equals(Set.of("x", "y"))).isTrue();
  }

  @Test
  void addThrowsUnsupportedOperationException() {
    final ReloadableSet<String> reloadableSet = new ReloadableSet<>(Set.of());

    assertThatThrownBy(() -> reloadableSet.add("b"))
        .isInstanceOf(UnsupportedOperationException.class)
        .hasMessageContaining("immutable");
  }

  @Test
  void removeThrowsUnsupportedOperationException() {
    final ReloadableSet<String> reloadableSet = new ReloadableSet<>(Set.of("a"));

    assertThat(reloadableSet.contains("a")).isTrue();

    assertThatThrownBy(() -> reloadableSet.remove("a"))
        .isInstanceOf(UnsupportedOperationException.class)
        .hasMessageContaining("immutable");
  }

  @Test
  void addAllThrowsUnsupportedOperationException() {
    final ReloadableSet<String> reloadableSet = new ReloadableSet<>(Set.of());

    assertThatThrownBy(() -> reloadableSet.addAll(Set.of("b", "c")))
        .isInstanceOf(UnsupportedOperationException.class)
        .hasMessageContaining("immutable");
  }

  @Test
  void retainAllThrowsUnsupportedOperationException() {
    final ReloadableSet<String> reloadableSet = new ReloadableSet<>(Set.of("a", "b"));

    assertThatThrownBy(() -> reloadableSet.retainAll(Set.of("a")))
        .isInstanceOf(UnsupportedOperationException.class)
        .hasMessageContaining("immutable");
  }

  @Test
  void removeAllThrowsUnsupportedOperationException() {
    final ReloadableSet<String> reloadableSet = new ReloadableSet<>(Set.of("a"));

    assertThat(reloadableSet.contains("a")).isTrue();

    assertThatThrownBy(() -> reloadableSet.removeAll(Set.of("a")))
        .isInstanceOf(UnsupportedOperationException.class)
        .hasMessageContaining("immutable");
  }

  @Test
  void clearThrowsUnsupportedOperationException() {
    final ReloadableSet<String> reloadableSet = new ReloadableSet<>(Set.of("a"));

    assertThatThrownBy(reloadableSet::clear)
        .isInstanceOf(UnsupportedOperationException.class)
        .hasMessageContaining("immutable");
  }

  @Test
  void concurrentIteratorsAlwaysSeeConsistentSnapshot() throws InterruptedException {
    // All swapped sets have exactly 3 elements — an iterator snapshot should never see any other
    // count
    final ReloadableSet<Integer> reloadableSet = new ReloadableSet<>(Set.of(1, 2, 3));

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
                  // Writer thread: swap to a new set of size 3
                  reloadableSet.swap(Set.of(j * 3, j * 3 + 1, j * 3 + 2));
                } else {
                  // Reader threads: iterator must be a consistent snapshot of exactly 3 elements
                  int count = 0;
                  for (Iterator<Integer> it = reloadableSet.iterator(); it.hasNext(); it.next()) {
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
    // All swapped sets have exactly 3 elements — a reader should never see any other size
    final ReloadableSet<Integer> reloadableSet = new ReloadableSet<>(Set.of(1, 2, 3));

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
                  // Writer thread: swap to a new set of size 3
                  reloadableSet.swap(Set.of(j * 3, j * 3 + 1, j * 3 + 2));
                } else {
                  // Reader threads: size must always be 3
                  int size = reloadableSet.size();
                  if (size != 3) {
                    errorCount.incrementAndGet();
                  }
                  // contains should never throw
                  reloadableSet.contains(j);
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
