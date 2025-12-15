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

package net.consensys.linea.plugins.rpc;

import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ConcurrentMap;

public class RequestLimiterDispatcher {
  public static final String SINGLE_INSTANCE_REQUEST_LIMITER_KEY = "single-instance-request-limit";
  private static final ConcurrentMap<String, RequestLimiter> ENDPOINT_LIMITER_MAP =
      new ConcurrentHashMap<>();

  public static RequestLimiter getLimiter(final String serviceKey) {
    return ENDPOINT_LIMITER_MAP.get(serviceKey);
  }

  public static void setLimiterIfMissing(
      final String serviceKey, final int concurrentRequestsLimit) {
    ENDPOINT_LIMITER_MAP.putIfAbsent(
        serviceKey,
        RequestLimiter.builder().concurrentRequestsCount(concurrentRequestsLimit).build());
  }
}
