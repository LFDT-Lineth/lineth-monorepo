const BASE_URL = "https://poh-api.linea.build/";

async function fetchWithBackoff(url: string, opts?: RequestInit, maxRetries = 5): Promise<Response> {
  let attempt = 0;
  // eslint-disable-next-line no-constant-condition
  while (true) {
    const res = await fetch(url, opts);
    if (res.status !== 429 || attempt >= maxRetries) return res;

    const retryAfterHeader = res.headers.get("Retry-After");
    const retryAfterMs = retryAfterHeader ? Number(retryAfterHeader) * 1000 : undefined;

    const base = 300;
    const jitter = Math.floor(Math.random() * 200);
    const backoffMs = retryAfterMs ?? base * 2 ** attempt + jitter;

    await new Promise((r) => setTimeout(r, backoffMs));
    attempt++;
  }
}

export async function checkPoh(address: string): Promise<boolean> {
  const url = BASE_URL + `poh/v2/${address}`;
  const res = await fetchWithBackoff(url);

  if (!res.ok) throw new Error(`HTTP ${res.status}`);

  const text = (await res.text()).trim();
  return text === "true";
}
