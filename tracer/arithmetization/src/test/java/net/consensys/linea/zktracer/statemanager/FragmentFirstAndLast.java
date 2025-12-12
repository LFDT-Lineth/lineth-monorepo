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

package net.consensys.linea.zktracer.statemanager;

import lombok.Getter;
import lombok.Setter;

/* FragmentFirstAndLast stores the first and last fragments relevant to the state manager in the
 * current transaction segment (they will be either account fragments or storage fragments). */
@Getter
@Setter
public class FragmentFirstAndLast<TraceFragment> {
  TraceFragment first;
  TraceFragment last;
  int firstDom, firstSub;
  int lastDom, lastSub;

  public FragmentFirstAndLast(
      TraceFragment first,
      TraceFragment last,
      int firstDom,
      int firstSub,
      int lastDom,
      int lastSub) {
    this.first = first;
    this.last = last;
    this.firstDom = firstDom;
    this.firstSub = firstSub;
    this.lastDom = lastDom;
    this.lastSub = lastSub;
  }

  public static boolean strictlySmallerStamps(
      int firstDom, int firstSub, int lastDom, int lastSub) {
    return firstDom < lastDom || (firstDom == lastDom && firstSub > lastSub);
  }

  public FragmentFirstAndLast<TraceFragment> copy() {
    return new FragmentFirstAndLast<TraceFragment>(
        this.first, this.last, this.firstDom, this.firstSub, this.lastDom, this.lastSub);
  }
}
