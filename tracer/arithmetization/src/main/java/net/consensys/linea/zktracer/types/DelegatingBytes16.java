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

import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.DelegatingBytes;

/** Holds and delegates all operations to its inner bytes field of exactly 16 bytes. */
public class DelegatingBytes16 extends DelegatingBytes implements Bytes16 {
  protected DelegatingBytes16(Bytes delegate) {
    super(delegate);
  }

  @Override
  public int size() {
    return Bytes16.SIZE;
  }

  @Override
  public Bytes16 copy() {
    return Bytes16.wrap(toArray());
  }

  @Override
  public MutableBytes16 mutableCopy() {
    return MutableBytes16.wrap(toArray());
  }
}
