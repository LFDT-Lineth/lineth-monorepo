const ALLOWED_ABSOLUTE_PROTOCOLS = new Set(["http:", "https:"]);

export function sanitizeAbsoluteHttpUrl(url?: string): string | undefined {
  if (!url) {
    return undefined;
  }

  try {
    const parsedUrl = new URL(url);
    return ALLOWED_ABSOLUTE_PROTOCOLS.has(parsedUrl.protocol) ? parsedUrl.toString() : undefined;
  } catch {
    return undefined;
  }
}

export function buildExplorerUrl(
  baseUrl: string | undefined,
  path: "address" | "tx",
  value: string,
): string | undefined {
  const safeBaseUrl = sanitizeAbsoluteHttpUrl(baseUrl);
  if (!safeBaseUrl) {
    return undefined;
  }

  const explorerUrl = new URL(safeBaseUrl);
  explorerUrl.pathname = `/${path}/${encodeURIComponent(value)}`;
  explorerUrl.search = "";
  explorerUrl.hash = "";
  return explorerUrl.toString();
}
