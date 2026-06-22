/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea.config;

import java.util.Collection;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.atomic.AtomicReference;
import javax.annotation.Nonnull;

/**
 * A thread-safe Map wrapper that supports atomic swapping of the underlying map for configuration
 * reload purposes. Implements Map&lt;K,V&gt; for drop-in compatibility.
 *
 * <p>This class is immutable from the perspective of Map operations - all mutation methods throw
 * {@link UnsupportedOperationException}. The underlying map can only be changed via {@link
 * #swap(Map)}.
 *
 * <p><b>Thread Safety:</b> All read operations delegate to the current underlying map obtained via
 * {@link AtomicReference#get()}. This means that iterators and collection views (from methods like
 * {@link #keySet()}, {@link #values()}, {@link #entrySet()}) represent a snapshot at the time of
 * the call and will not reflect subsequent swaps. This is the intended behavior for configuration
 * reload scenarios.
 *
 * <p>Example usage:
 *
 * <pre>{@code
 * // Create with an initial map
 * ReloadableMap<Address, Set<EventFilter>> deniedEvents = new ReloadableMap<>(
 *     parseEventsDenyList(config.eventsDenyListPath())
 * );
 *
 * // Use as a Map (implements Map<K,V>)
 * Set<EventFilter> filters = deniedEvents.get(address);
 *
 * // Swap with a new value when configuration changes
 * deniedEvents.swap(newMap);
 * }</pre>
 *
 * @param <K> the type of keys in this map
 * @param <V> the type of values in this map
 */
public class ReloadableMap<K, V> implements Map<K, V> {
  private final AtomicReference<Map<K, V>> delegate;

  /**
   * Creates a new ReloadableMap with an initial map value.
   *
   * @param initial the initial map contents
   */
  public ReloadableMap(final Map<K, V> initial) {
    this.delegate = new AtomicReference<>(initial);
  }

  /**
   * Swap the underlying map with a new value. This is useful for atomic batch updates where all
   * values are loaded first, then swapped together.
   *
   * @param newValue the new map to use
   */
  public void swap(final Map<K, V> newValue) {
    delegate.set(newValue);
  }

  // Delegate all Map read methods to the underlying map

  @Override
  public int size() {
    return delegate.get().size();
  }

  @Override
  public boolean isEmpty() {
    return delegate.get().isEmpty();
  }

  @Override
  public boolean containsKey(final Object key) {
    return delegate.get().containsKey(key);
  }

  @Override
  public boolean containsValue(final Object value) {
    return delegate.get().containsValue(value);
  }

  @Override
  public V get(final Object key) {
    return delegate.get().get(key);
  }

  @Nonnull
  @Override
  public Set<K> keySet() {
    return delegate.get().keySet();
  }

  @Nonnull
  @Override
  public Collection<V> values() {
    return delegate.get().values();
  }

  @Nonnull
  @Override
  public Set<Entry<K, V>> entrySet() {
    return delegate.get().entrySet();
  }

  @Override
  public boolean equals(final Object o) {
    if (this == o) return true;
    if (o instanceof ReloadableMap<?, ?> other) {
      return delegate.get().equals(other.delegate.get());
    }
    return delegate.get().equals(o);
  }

  @Override
  public int hashCode() {
    return delegate.get().hashCode();
  }

  // Mutation methods throw UnsupportedOperationException (immutable view)

  @Override
  public V put(final K key, final V value) {
    throw new UnsupportedOperationException(
        "ReloadableMap is immutable; use swap() to change contents");
  }

  @Override
  public V remove(final Object key) {
    throw new UnsupportedOperationException(
        "ReloadableMap is immutable; use swap() to change contents");
  }

  @Override
  public void putAll(@Nonnull final Map<? extends K, ? extends V> m) {
    throw new UnsupportedOperationException(
        "ReloadableMap is immutable; use swap() to change contents");
  }

  @Override
  public void clear() {
    throw new UnsupportedOperationException(
        "ReloadableMap is immutable; use swap() to change contents");
  }
}
