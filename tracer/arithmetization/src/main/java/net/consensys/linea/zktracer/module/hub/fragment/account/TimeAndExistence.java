/*
 * Copyright ConsenSys Inc.
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

package net.consensys.linea.zktracer.module.hub.fragment.account;

public record TimeAndExistence(int domStamp, int subStamp, boolean hadCode) {

  public boolean needsUpdate(TimeAndExistence other) {
    if (other.hadCode() == this.hadCode) {
      return false;
    }

    if (other.domStamp < this.domStamp) {
      return true;
    } else if (other.domStamp == this.domStamp) {
      return other.subStamp > this.subStamp;
    } else {
      return false;
    }
  }
}
