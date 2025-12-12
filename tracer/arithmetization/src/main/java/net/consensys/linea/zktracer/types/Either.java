/*
 * Copyright Consensys Software Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package net.consensys.linea.zktracer.types;

import java.util.function.Function;

public final class Either<L, R> {
  public static <L, R> Either<L, R> left(L value) {
    return new Either<>(value, null);
  }

  public static <L, R> Either<L, R> right(R value) {
    return new Either<>(null, value);
  }

  private final L left;
  private final R right;

  private Either(L l, R r) {
    left = l;
    right = r;
  }

  public <T> T map(Function<? super L, ? extends T> lFunc, Function<? super R, ? extends T> rFunc) {
    if (left != null) {
      return lFunc.apply(left);
    } else {
      return rFunc.apply(right);
    }
  }

  public <T> Either<T, R> mapLeft(Function<? super L, ? extends T> lFunc) {
    return new Either<>(lFunc.apply(left), right);
  }

  public <T> Either<L, T> mapRight(Function<? super R, ? extends T> rFunc) {
    return new Either<>(left, rFunc.apply(right));
  }
}
