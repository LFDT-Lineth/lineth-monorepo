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

public record TimeAndExistence(int domStamp, int subStamp, boolean hadCode, boolean seenInTxAuth) {

  public boolean needsUpdate(TimeAndExistence next) {
    // if next stems from a TX_AUTH row, we don't update
    if (next.seenInTxAuth) {
      return false;
    }
    // below, next DOES NOT stem from a TX_AUTH row

    // if this DOES, we update
    if (this.seenInTxAuth) {
      return true;
    }

    // NOTE. Beyond this point:
    // this.txAuthAccountFragment == false
    // next.txAuthAccountFragment == false

    if (next.hadCode() == this.hadCode) {
      return false;
    }

    if (next.domStamp < this.domStamp) {
      return true;
    } else if (next.domStamp == this.domStamp) {
      return next.subStamp > this.subStamp;
    } else {
      return false;
    }
  }

  public boolean tracedHadCode() {
    if (seenInTxAuth) {
      return false;
    }
    return hadCode;
  }
}
