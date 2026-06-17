const HASH_RE = /^0x[a-fA-F0-9]{64}$/;
const ADDRESS_RE = /^0x[a-fA-F0-9]{40}$/;
const PRIVATE_KEY_RE = /^0x[a-fA-F0-9]{64}$/;
const HEX_BYTES_RE = /^0x([a-fA-F0-9]{2})*$/;
const UINT_RE = /^[0-9]+$/;

function required(env, name) {
  const value = env[name];
  if (!value) {
    throw new Error(`${name} is required`);
  }
  return value;
}

function requireMatch(env, name, pattern, label) {
  const value = required(env, name);
  if (!pattern.test(value)) {
    throw new Error(`${name} must be ${label}`);
  }
  return value;
}

function requireUint(env, name) {
  return requireMatch(env, name, UINT_RE, "a decimal unsigned integer");
}

function asBigInt(env, name) {
  return BigInt(requireUint(env, name));
}

function asChainId(env, name) {
  const value = Number(requireUint(env, name));
  if (!Number.isSafeInteger(value) || value <= 0) {
    throw new Error(`${name} must be a safe positive integer`);
  }
  return value;
}

function chain(id, name, rpcUrl) {
  return {
    id,
    name,
    nativeCurrency: { name: "Ether", symbol: "ETH", decimals: 18 },
    rpcUrls: { default: { http: [rpcUrl] } },
  };
}

export function sanitizeError(error, secrets = []) {
  let message = error instanceof Error ? error.message : String(error);
  for (const secret of secrets) {
    if (!secret) {
      continue;
    }
    message = message.split(secret).join("[REDACTED]");
    message = message.split(secret.toLowerCase()).join("[REDACTED]");
    message = message.split(secret.toUpperCase()).join("[REDACTED]");
  }
  return message;
}

export async function loadDefaultDeps() {
  const { createPublicClient, createWalletClient, http, zeroAddress } = await import("viem");
  const { privateKeyToAccount } = await import("viem/accounts");
  const { claimOnL1, getL2ToL1MessageStatus, getMessageProof } = await import("@consensys/linea-sdk-viem");

  return {
    createPublicClient,
    createWalletClient,
    http,
    zeroAddress,
    privateKeyToAccount,
    claimOnL1,
    getL2ToL1MessageStatus,
    getMessageProof,
  };
}

export async function claimL2ToL1(env = process.env, deps) {
  const signerPrivateKey = requireMatch(env, "L1_SIGNER_PRIVATE_KEY", PRIVATE_KEY_RE, "a 32-byte hex private key");

  try {
    const resolvedDeps = deps ?? (await loadDefaultDeps());
    const l1RpcUrl = required(env, "L1_RPC_URL");
    const l2RpcUrl = required(env, "L2_RPC_URL");
    const l1Chain = chain(asChainId(env, "SMOKE_L1_CHAIN_ID"), "sepolia", l1RpcUrl);
    const l2Chain = chain(asChainId(env, "SMOKE_L2_CHAIN_ID"), "local-linea", l2RpcUrl);
    const lineaRollupAddress = requireMatch(env, "SMOKE_LINEA_ROLLUP_ADDRESS", ADDRESS_RE, "an address");
    const l2MessageServiceAddress = requireMatch(
      env,
      "SMOKE_L2_MESSAGE_SERVICE_ADDRESS",
      ADDRESS_RE,
      "an address",
    );
    const messageHash = requireMatch(env, "SMOKE_MESSAGE_HASH", HASH_RE, "a 32-byte hash");
    const from = requireMatch(env, "SMOKE_MESSAGE_SENDER", ADDRESS_RE, "an address");
    const to = requireMatch(env, "SMOKE_DESTINATION", ADDRESS_RE, "an address");
    const fee = asBigInt(env, "SMOKE_FEE");
    const value = asBigInt(env, "SMOKE_VALUE");
    const messageNonce = asBigInt(env, "SMOKE_MESSAGE_NONCE");
    const calldata = requireMatch(env, "SMOKE_CALLDATA", HEX_BYTES_RE, "hex bytes");
    const sentBlockNumber = asBigInt(env, "SMOKE_SENT_BLOCK_NUMBER");

    const account = resolvedDeps.privateKeyToAccount(signerPrivateKey);
    const l1PublicClient = resolvedDeps.createPublicClient({
      chain: l1Chain,
      transport: resolvedDeps.http(l1RpcUrl),
    });
    const l1WalletClient = resolvedDeps.createWalletClient({
      account,
      chain: l1Chain,
      transport: resolvedDeps.http(l1RpcUrl),
    });
    const l2PublicClient = resolvedDeps.createPublicClient({
      chain: l2Chain,
      transport: resolvedDeps.http(l2RpcUrl),
    });
    const l2LogsBlockRange = {
      fromBlock: sentBlockNumber,
      toBlock: sentBlockNumber,
    };

    const common = {
      l2Client: l2PublicClient,
      messageHash,
      lineaRollupAddress,
      l2MessageServiceAddress,
      l2LogsBlockRange,
    };

    const status = await resolvedDeps.getL2ToL1MessageStatus(l1PublicClient, common);
    if (status !== "CLAIMABLE") {
      throw new Error(`L2->L1 message is ${status}, not CLAIMABLE`);
    }

    const messageProof = await resolvedDeps.getMessageProof(l1PublicClient, common);
    const claimTxHash = await resolvedDeps.claimOnL1(l1WalletClient, {
      from,
      to,
      fee,
      value,
      messageNonce,
      calldata,
      feeRecipient: resolvedDeps.zeroAddress,
      messageProof,
      lineaRollupAddress,
    });

    return {
      status,
      claimTxHash,
      proofRoot: messageProof.root,
      proofLeafIndex:
        typeof messageProof.leafIndex === "bigint" ? Number(messageProof.leafIndex) : messageProof.leafIndex,
      proofLength: messageProof.proof.length,
      claimant: account.address,
    };
  } catch (error) {
    throw new Error(sanitizeError(error, [signerPrivateKey]));
  }
}

async function main() {
  const result = await claimL2ToL1(process.env);
  console.log(JSON.stringify(result));
}

if (process.env.CLAIM_L2_TO_L1_DISABLE_MAIN !== "true") {
  main().catch((error) => {
    console.error(`[claim-l2-to-l1] ERROR: ${sanitizeError(error, [process.env.L1_SIGNER_PRIVATE_KEY])}`);
    process.exit(1);
  });
}
