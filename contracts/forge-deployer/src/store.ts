import { mkdir, readFile, rename, writeFile } from "node:fs/promises";
import https from "node:https";
import path from "node:path";
import { setTimeout as wait } from "node:timers/promises";

import { L2_DETERMINISTIC_PROXY_FACTORY_KEY } from "./address-plan";
import { DeploymentCheckpoint, parseCheckpoint, touchCheckpoint } from "./checkpoint";
import { DeployerConfig } from "./config";

export const DEFAULT_CONFIG_MAP_NAME = "lineth-contract-addresses";
const CHECKPOINT_DATA_KEY = "checkpoint.json";
const SERVICE_ACCOUNT_PATH = "/var/run/secrets/kubernetes.io/serviceaccount";
const KUBERNETES_WRITE_ATTEMPTS = 5;
const KUBERNETES_REQUEST_TIMEOUT_MS = 10_000;
const KUBERNETES_RETRY_BASE_DELAY_MS = 100;

export interface CheckpointStore {
  load(): Promise<DeploymentCheckpoint | undefined>;
  save(checkpoint: DeploymentCheckpoint): Promise<void>;
}

function completedAddress(checkpoint: DeploymentCheckpoint, key: string): string | undefined {
  return checkpoint.deployments[key]?.address;
}

export function checkpointToConfigMapData(checkpoint: DeploymentCheckpoint): Record<string, string> {
  const expectedAddresses = Object.fromEntries(
    Object.entries(checkpoint.expectedDeployments).map(([key, deployment]) => [key, deployment.expectedAddress]),
  );
  const serializedCheckpoint = JSON.stringify(checkpoint, null, 2);
  const data: Record<string, string> = {
    [CHECKPOINT_DATA_KEY]: serializedCheckpoint,
    "addresses.json": serializedCheckpoint,
    "expected-addresses.json": JSON.stringify(expectedAddresses, null, 2),
  };

  const publicAddresses: Array<[string, string | undefined]> = [
    ["lineth-rollup", completedAddress(checkpoint, "l1-rollup.proxy")],
    ["l2-message-service", completedAddress(checkpoint, "l2-message-service.proxy")],
    ["l1-token-bridge", completedAddress(checkpoint, "l1-token-bridge.proxy")],
    ["l2-token-bridge", completedAddress(checkpoint, "l2-token-bridge.proxy")],
    ["deterministic-deployment-proxy", completedAddress(checkpoint, L2_DETERMINISTIC_PROXY_FACTORY_KEY)],
  ];
  for (const [key, value] of publicAddresses) {
    if (value !== undefined) data[key] = value;
  }

  return data;
}

export function configMapDataPatch(resourceVersion: string, data: Record<string, string>): unknown[] {
  return [
    { op: "test", path: "/metadata/resourceVersion", value: resourceVersion },
    { op: "add", path: "/data", value: data },
  ];
}

export function configMapContainsData(
  configMap: { data?: Record<string, string> },
  expectedData: Record<string, string>,
): boolean {
  const actualData = configMap.data ?? {};
  const expectedEntries = Object.entries(expectedData);
  return (
    Object.keys(actualData).length === expectedEntries.length &&
    expectedEntries.every(([key, value]) => actualData[key] === value)
  );
}

export function isRetryableKubernetesStatus(statusCode: number): boolean {
  return statusCode === 429 || [500, 502, 503, 504].includes(statusCode);
}

export function isKubernetesConflictStatus(statusCode: number): boolean {
  return statusCode === 409 || statusCode === 422;
}

export class FileCheckpointStore implements CheckpointStore {
  public constructor(private readonly filePath: string) {}

  public async load(): Promise<DeploymentCheckpoint | undefined> {
    try {
      return parseCheckpoint(await readFile(this.filePath, "utf8"));
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code === "ENOENT") return undefined;
      throw error;
    }
  }

  public async save(checkpoint: DeploymentCheckpoint): Promise<void> {
    touchCheckpoint(checkpoint);
    await mkdir(path.dirname(this.filePath), { recursive: true });
    const temporaryPath = `${this.filePath}.tmp`;
    await writeFile(temporaryPath, JSON.stringify(checkpoint, null, 2), { mode: 0o600 });
    await rename(temporaryPath, this.filePath);
  }
}

interface KubernetesConfigMap {
  metadata?: {
    resourceVersion?: string;
  };
  data?: Record<string, string>;
}

interface KubernetesResponse {
  statusCode: number;
  body: string;
}

class KubernetesHttpError extends Error {
  public constructor(
    message: string,
    public readonly statusCode: number,
  ) {
    super(message);
  }
}

class KubernetesTransportError extends Error {}

export class KubernetesCheckpointStore implements CheckpointStore {
  private readonly host: string;
  private readonly port: number;
  private readonly namespace: string;
  private readonly name: string;
  private readonly token: string;
  private readonly certificateAuthority: Buffer;
  private resourceVersion: string | undefined;

  private constructor(input: {
    host: string;
    port: number;
    namespace: string;
    name: string;
    token: string;
    certificateAuthority: Buffer;
  }) {
    this.host = input.host;
    this.port = input.port;
    this.namespace = input.namespace;
    this.name = input.name;
    this.token = input.token;
    this.certificateAuthority = input.certificateAuthority;
  }

  public static async fromEnvironment(env: NodeJS.ProcessEnv = process.env): Promise<KubernetesCheckpointStore> {
    const host = env.KUBERNETES_SERVICE_HOST;
    if (!host) throw new Error("KUBERNETES_SERVICE_HOST is required unless CHECKPOINT_FILE is set");
    const port = Number(env.KUBERNETES_SERVICE_PORT_HTTPS ?? "443");
    if (!Number.isSafeInteger(port) || port <= 0) throw new Error("KUBERNETES_SERVICE_PORT_HTTPS is invalid");
    const [token, namespace, certificateAuthority] = await Promise.all([
      readFile(path.join(SERVICE_ACCOUNT_PATH, "token"), "utf8"),
      env.POD_NAMESPACE
        ? Promise.resolve(env.POD_NAMESPACE)
        : readFile(path.join(SERVICE_ACCOUNT_PATH, "namespace"), "utf8"),
      readFile(path.join(SERVICE_ACCOUNT_PATH, "ca.crt")),
    ]);
    return new KubernetesCheckpointStore({
      host,
      port,
      namespace: namespace.trim(),
      name: env.CHECKPOINT_CONFIG_MAP_NAME?.trim() || DEFAULT_CONFIG_MAP_NAME,
      token: token.trim(),
      certificateAuthority,
    });
  }

  private resourcePath(): string {
    return `/api/v1/namespaces/${encodeURIComponent(this.namespace)}/configmaps/${encodeURIComponent(this.name)}`;
  }

  private async request(method: string, requestPath: string, body?: unknown): Promise<KubernetesResponse> {
    const payload = body === undefined ? undefined : JSON.stringify(body);
    return await new Promise<KubernetesResponse>((resolve, reject) => {
      const request = https.request(
        {
          hostname: this.host,
          port: this.port,
          path: requestPath,
          method,
          ca: this.certificateAuthority,
          headers: {
            authorization: `Bearer ${this.token}`,
            accept: "application/json",
            ...(payload === undefined
              ? {}
              : {
                  "content-type": method === "PATCH" ? "application/json-patch+json" : "application/json",
                  "content-length": Buffer.byteLength(payload).toString(),
                }),
          },
        },
        (response) => {
          let responseBody = "";
          response.setEncoding("utf8");
          response.on("data", (chunk: string) => {
            responseBody += chunk;
          });
          response.on("end", () => {
            resolve({ statusCode: response.statusCode ?? 0, body: responseBody });
          });
        },
      );
      request.setTimeout(KUBERNETES_REQUEST_TIMEOUT_MS, () => {
        request.destroy(
          new KubernetesTransportError(`Kubernetes API request timed out after ${KUBERNETES_REQUEST_TIMEOUT_MS}ms`),
        );
      });
      request.on("error", (error) => {
        reject(
          error instanceof KubernetesTransportError
            ? error
            : new KubernetesTransportError(`Kubernetes API request failed: ${error.message}`, { cause: error }),
        );
      });
      if (payload !== undefined) request.write(payload);
      request.end();
    });
  }

  private async getConfigMap(): Promise<KubernetesConfigMap | undefined> {
    const response = await this.request("GET", this.resourcePath());
    if (response.statusCode === 404) return undefined;
    if (response.statusCode !== 200) {
      throw new KubernetesHttpError(
        `Kubernetes ConfigMap read failed with HTTP ${response.statusCode}: ${response.body}`,
        response.statusCode,
      );
    }
    return JSON.parse(response.body) as KubernetesConfigMap;
  }

  public async load(): Promise<DeploymentCheckpoint | undefined> {
    const configMap = await this.getConfigMap();
    this.resourceVersion = configMap?.metadata?.resourceVersion;
    const rawCheckpoint = configMap?.data?.[CHECKPOINT_DATA_KEY];
    return rawCheckpoint === undefined ? undefined : parseCheckpoint(rawCheckpoint);
  }

  public async save(checkpoint: DeploymentCheckpoint): Promise<void> {
    touchCheckpoint(checkpoint);
    const desiredData = checkpointToConfigMapData(checkpoint);
    for (let attempt = 1; attempt <= KUBERNETES_WRITE_ATTEMPTS; attempt += 1) {
      try {
        if (!this.resourceVersion) {
          const existing = await this.getConfigMap();
          if (!existing) {
            throw new KubernetesHttpError(
              `Kubernetes ConfigMap ${this.namespace}/${this.name} is missing; the chart must create its placeholder`,
              404,
            );
          }
          this.resourceVersion = existing.metadata?.resourceVersion;
        }
        if (!this.resourceVersion) {
          throw new Error(`Kubernetes ConfigMap ${this.namespace}/${this.name} has no resourceVersion`);
        }
        const response = await this.request(
          "PATCH",
          this.resourcePath(),
          configMapDataPatch(this.resourceVersion, desiredData),
        );
        if (response.statusCode === 200) {
          const updated = JSON.parse(response.body) as KubernetesConfigMap;
          if (!updated.metadata?.resourceVersion) {
            throw new Error(`Kubernetes ConfigMap update response has no resourceVersion`);
          }
          this.resourceVersion = updated.metadata.resourceVersion;
          return;
        }
        if (isKubernetesConflictStatus(response.statusCode)) {
          const current = await this.getConfigMap();
          if (current && configMapContainsData(current, desiredData) && current.metadata?.resourceVersion) {
            this.resourceVersion = current.metadata.resourceVersion;
            return;
          }
          throw new KubernetesHttpError(
            `Kubernetes ConfigMap ${this.namespace}/${this.name} changed concurrently; refusing to overwrite it`,
            response.statusCode,
          );
        }
        throw new KubernetesHttpError(
          `Kubernetes ConfigMap write failed with HTTP ${response.statusCode}: ${response.body}`,
          response.statusCode,
        );
      } catch (error) {
        const shouldRetry =
          (error instanceof KubernetesTransportError ||
            (error instanceof KubernetesHttpError && isRetryableKubernetesStatus(error.statusCode))) &&
          attempt < KUBERNETES_WRITE_ATTEMPTS;
        if (!shouldRetry) throw error;
        await wait(KUBERNETES_RETRY_BASE_DELAY_MS * 2 ** (attempt - 1));
      }
    }
    throw new Error("Kubernetes ConfigMap write attempts exhausted");
  }
}

export async function createCheckpointStore(config: DeployerConfig): Promise<CheckpointStore> {
  return config.checkpointFile
    ? new FileCheckpointStore(config.checkpointFile)
    : await KubernetesCheckpointStore.fromEnvironment();
}
