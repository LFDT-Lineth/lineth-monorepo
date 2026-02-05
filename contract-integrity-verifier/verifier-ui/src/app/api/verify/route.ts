/**
 * Verification API Routes
 *
 * NOTE: SERVER MODE ONLY
 * These routes are only used when NEXT_PUBLIC_STORAGE_MODE=server.
 * In client mode (default), verification runs directly in the browser
 * using the ClientVerifierService with viem adapter.
 *
 * To enable server mode in the future, implement ServerVerifierService in
 * src/services/server-verifier-service.ts to use these API routes.
 */

// Required for static export - makes these routes return 404 in static builds
// eslint-disable-next-line @typescript-eslint/no-unused-vars
export const dynamic = "force-static";

import { NextResponse } from "next/server";
import { Verifier } from "@consensys/linea-contract-integrity-verifier";
import type { VerifierConfig } from "@consensys/linea-contract-integrity-verifier";
import { EthersAdapter } from "@consensys/linea-contract-integrity-verifier-ethers";
import { ViemAdapter } from "@consensys/linea-contract-integrity-verifier-viem";
import { getSession } from "@/lib/session";
import { verifyRequestSchema } from "@/lib/validation";
import {
  rewriteConfigPaths,
  interpolateEnvVarsInContent,
  parseMarkdownConfig,
  preprocessJsonWithEnvVars,
} from "@/lib/config-parser";
import type { ApiError, VerifyResponse } from "@/types";
import type { ContractVerificationResult, VerificationSummary } from "@consensys/linea-contract-integrity-verifier";
import { dirname } from "path";

// Type for adapters
type Web3Adapter = InstanceType<typeof EthersAdapter> | InstanceType<typeof ViemAdapter>;

// POST /api/verify - Run verification
export async function POST(request: Request): Promise<NextResponse<VerifyResponse | ApiError>> {
  try {
    const body = await request.json();

    // Validate request
    const validation = verifyRequestSchema.safeParse(body);
    if (!validation.success) {
      return NextResponse.json(
        {
          code: "INVALID_CONFIG",
          message: validation.error.message,
        },
        { status: 400 },
      );
    }

    const { sessionId, adapter, envVars, options } = validation.data;

    // Get session
    const session = await getSession(sessionId);
    if (!session) {
      return NextResponse.json({ code: "SESSION_EXPIRED", message: "Session not found or expired" }, { status: 404 });
    }

    if (!session.config) {
      return NextResponse.json({ code: "INVALID_CONFIG", message: "No config file uploaded" }, { status: 400 });
    }

    // Check all required files are uploaded
    const missingFiles = session.config.requiredFiles.filter((f) => !session.fileMap[f.path]);
    if (missingFiles.length > 0) {
      return NextResponse.json(
        {
          code: "MISSING_FILE",
          message: `Missing required files: ${missingFiles.map((f) => f.path).join(", ")}`,
        },
        { status: 400 },
      );
    }

    // Check all env vars are provided
    const missingEnvVars = session.config.envVars.filter((v) => !envVars[v] || envVars[v].trim() === "");
    if (missingEnvVars.length > 0) {
      return NextResponse.json(
        {
          code: "MISSING_ENV_VAR",
          message: `Missing environment variables: ${missingEnvVars.join(", ")}`,
        },
        { status: 400 },
      );
    }

    // Interpolate environment variables and parse config based on format
    let config: VerifierConfig;
    try {
      // First interpolate env vars in the raw content
      const interpolatedContent = interpolateEnvVarsInContent(session.config.rawContent, envVars);

      // Parse based on format
      if (session.config.format === "markdown") {
        config = parseMarkdownConfig(interpolatedContent);
      } else {
        // For JSON, we need to preprocess to handle any remaining placeholder syntax
        const preprocessed = preprocessJsonWithEnvVars(interpolatedContent);
        config = JSON.parse(preprocessed) as VerifierConfig;
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : "Failed to parse config";
      return NextResponse.json({ code: "INVALID_CONFIG", message }, { status: 400 });
    }

    // Rewrite config paths to uploaded locations
    config = rewriteConfigPaths(config, session.fileMap);

    // Create adapters for each chain based on user's library choice
    const adapters = new Map<string, Web3Adapter>();

    for (const [chainName, chainConfig] of Object.entries(config.chains)) {
      if (chainConfig.rpcUrl) {
        if (adapter === "ethers") {
          adapters.set(chainName, new EthersAdapter({ rpcUrl: chainConfig.rpcUrl, chainId: chainConfig.chainId }));
        } else {
          adapters.set(chainName, new ViemAdapter({ rpcUrl: chainConfig.rpcUrl, chainId: chainConfig.chainId }));
        }
      }
    }

    // Get config directory for relative path resolution
    // Since files are stored in session directory, use that as config dir
    const configDir = dirname(Object.values(session.fileMap)[0] || ".");

    // Filter contracts if specified
    let contractsToVerify = config.contracts;
    if (options.contractFilter) {
      contractsToVerify = contractsToVerify.filter(
        (c) => c.name.toLowerCase() === options.contractFilter!.toLowerCase(),
      );
    }
    if (options.chainFilter) {
      contractsToVerify = contractsToVerify.filter((c) => c.chain.toLowerCase() === options.chainFilter!.toLowerCase());
    }

    // Run verification for each contract
    const results: ContractVerificationResult[] = [];
    let passed = 0;
    let failed = 0;
    let warnings = 0;
    let skipped = 0;

    for (const contract of contractsToVerify) {
      const chainAdapter = adapters.get(contract.chain);
      if (!chainAdapter) {
        skipped++;
        continue;
      }

      const chain = config.chains[contract.chain];
      const verifier = new Verifier(chainAdapter);

      const result = await verifier.verifyContract(
        contract,
        chain,
        {
          verbose: options.verbose,
          skipBytecode: options.skipBytecode,
          skipAbi: options.skipAbi,
          skipState: options.skipState,
        },
        configDir,
      );

      results.push(result);

      // Count results
      if (result.error) {
        failed++;
      } else {
        const bytecodeStatus = result.bytecodeResult?.status;
        const abiStatus = result.abiResult?.status;
        const stateStatus = result.stateResult?.status;

        // Collect non-undefined statuses
        const statuses = [bytecodeStatus, abiStatus, stateStatus].filter(Boolean);

        if (bytecodeStatus === "fail" || abiStatus === "fail" || stateStatus === "fail") {
          failed++;
        } else if (bytecodeStatus === "warn" || abiStatus === "warn" || stateStatus === "warn") {
          warnings++;
        } else if (statuses.length > 0 && statuses.every((s) => s === "skip")) {
          // All checks were skipped
          skipped++;
        } else {
          passed++;
        }
      }
    }

    const summary: VerificationSummary = {
      total: contractsToVerify.length,
      passed,
      failed,
      warnings,
      skipped,
      results,
    };

    return NextResponse.json({ summary });
  } catch (error) {
    console.error("Verification failed:", error);

    // Check if it's an RPC error
    const message = error instanceof Error ? error.message : "Verification failed";
    const isRpcError =
      message.includes("RPC") ||
      message.includes("network") ||
      message.includes("timeout") ||
      message.includes("ECONNREFUSED");

    return NextResponse.json(
      {
        code: isRpcError ? "RPC_ERROR" : "VERIFICATION_FAILED",
        message,
      },
      { status: 500 },
    );
  }
}
