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

package net.consensys.linea.zktracer.bytestheta;

import org.apache.tuweni.bytes.Bytes;

/** Provides high and low parts of a {@link Bytes} object. */
public interface HighLowBytes {
  /**
   * Returns the high part of the bytes object.
   *
   * @return the high part of the bytes object
   */
  Bytes getHigh();

  /**
   * Returns the low part of the bytes object.
   *
   * @return the low part of the bytes object
   */
  Bytes getLow();
}
