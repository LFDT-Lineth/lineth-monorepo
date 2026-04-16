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
import java.util.Iterator;
import java.util.Set;
import java.util.concurrent.atomic.AtomicReference;
import javax.annotation.Nonnull;

/**
 * A thread-safe Set wrapper that supports atomic swapping of the underlying set for configuration
 * reload purposes. Implements Set&lt;T&gt; for drop-in compatibility.
 *
 * <p>This class is immutable from the perspective of Set operations - all mutation methods throw
 * {@link UnsupportedOperationException}. The underlying set can only be changed via {@link
 * #swap(Set)}.
 *
 * <p><b>Thread Safety:</b> All read operations delegate to the current underlying set obtained via
 * {@link AtomicReference#get()}. This means that iterators and collection views (from methods like
 * {@link #iterator()}, {@link #toArray()}) represent a snapshot at the time of the call and will
 * not reflect subsequent swaps. This is the intended behavior for configuration reload scenarios.
 *
 * <p>Example usage:
 *
 * <pre>{@code
 * // Create with an initial set
 * ReloadableSet<Address> deniedAddresses = new ReloadableSet<>(
 *     parseDeniedAddresses(config.denyListPath())
 * );
 *
 * // Use as a Set (implements Set<T>)
 * if (deniedAddresses.contains(address)) { ... }
 *
 * // Swap with a new value when configuration changes
 * deniedAddresses.swap(newSet);
 * }</pre>
 *
 * @param <T> the type of elements in this set
 */
public class ReloadableSet<T> implements Set<T> {
  private final AtomicReference<Set<T>> delegate;

  /**
   * Creates a new ReloadableSet with an initial set value.
   *
   * @param initial the initial set contents
   */
  public ReloadableSet(final Set<T> initial) {
    this.delegate = new AtomicReference<>(initial);
  }

  /**
   * Swap the underlying set with a new value. This is useful for atomic batch updates where all
   * values are loaded first, then swapped together.
   *
   * @param newValue the new set to use
   */
  public void swap(final Set<T> newValue) {
    delegate.set(newValue);
  }

  // Delegate all Set read methods to the underlying set

  @Override
  public int size() {
    return delegate.get().size();
  }

  @Override
  public boolean isEmpty() {
    return delegate.get().isEmpty();
  }

  @Override
  public boolean contains(final Object o) {
    return delegate.get().contains(o);
  }

  @Nonnull
  @Override
  public Iterator<T> iterator() {
    return delegate.get().iterator();
  }

  @Nonnull
  @Override
  public Object[] toArray() {
    return delegate.get().toArray();
  }

  @Nonnull
  @Override
  public <T1> T1[] toArray(@Nonnull final T1[] a) {
    return delegate.get().toArray(a);
  }

  @Override
  public boolean containsAll(@Nonnull final Collection<?> c) {
    return delegate.get().containsAll(c);
  }

  @Override
  public boolean equals(final Object o) {
    if (this == o) return true;
    if (o instanceof ReloadableSet<?> other) {
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
  public boolean add(final T t) {
    throw new UnsupportedOperationException(
        "ReloadableSet is immutable; use swap() to change contents");
  }

  @Override
  public boolean remove(final Object o) {
    throw new UnsupportedOperationException(
        "ReloadableSet is immutable; use swap() to change contents");
  }

  @Override
  public boolean addAll(@Nonnull final Collection<? extends T> c) {
    throw new UnsupportedOperationException(
        "ReloadableSet is immutable; use swap() to change contents");
  }

  @Override
  public boolean retainAll(@Nonnull final Collection<?> c) {
    throw new UnsupportedOperationException(
        "ReloadableSet is immutable; use swap() to change contents");
  }

  @Override
  public boolean removeAll(@Nonnull final Collection<?> c) {
    throw new UnsupportedOperationException(
        "ReloadableSet is immutable; use swap() to change contents");
  }

  @Override
  public void clear() {
    throw new UnsupportedOperationException(
        "ReloadableSet is immutable; use swap() to change contents");
  }
}
