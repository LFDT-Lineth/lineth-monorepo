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

import org.apache.tuweni.bytes.DelegatingMutableBytes;
import org.apache.tuweni.bytes.MutableBytes;

final class DelegatingMutableBytes16 extends DelegatingMutableBytes implements MutableBytes16 {

  final MutableBytes delegate;

  private DelegatingMutableBytes16(MutableBytes delegate) {
    super(delegate);
    this.delegate = delegate;
  }

  static MutableBytes16 delegateTo(MutableBytes value) {
    Checks.checkArgument(value.size() == SIZE, "Expected %s bytes but got %s", SIZE, value.size());
    return new DelegatingMutableBytes16(value);
  }

  @Override
  public Bytes16 copy() {
    return Bytes16.wrap(delegate.toArray());
  }

  @Override
  public MutableBytes16 mutableCopy() {
    return MutableBytes16.wrap(delegate.toArray());
  }
}
