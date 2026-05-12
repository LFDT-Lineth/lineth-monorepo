/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea.config;

import java.util.Map;
import lombok.Builder;
import net.consensys.linea.plugins.LineaOptionsConfiguration;

/** The Linea tracer line limit configuration. */
@Builder(toBuilder = true)
public record LineaTracerLineLimitConfiguration(
    String moduleLimitsFilePath, Map<String, Integer> moduleLimitsMap)
    implements LineaOptionsConfiguration {}
