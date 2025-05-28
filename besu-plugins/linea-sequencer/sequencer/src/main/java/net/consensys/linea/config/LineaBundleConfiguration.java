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

package net.consensys.linea.config;

import java.net.URL;
import java.util.Set;

import lombok.Builder;
import lombok.Getter;
import lombok.ToString;
import lombok.experimental.Accessors;
import net.consensys.linea.plugins.LineaOptionsConfiguration;

/** The Linea Bundle configuration. */
@Builder(toBuilder = true)
@Accessors(fluent = true)
@Getter
@ToString
public class LineaBundleConfiguration implements LineaOptionsConfiguration {
  private Set<URL> forwardUrls;
  private int retryDelayMillis;
  private int timeoutMillis;
}
