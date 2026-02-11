import type { Logger } from "winston";

export type RetryOptions = {
  retries: number;
  delayMs: number;
};

export class RetryPolicy {
  constructor(
    private readonly logger: Logger,
    private readonly options: RetryOptions,
  ) {}

  async execute<T>(fn: () => Promise<T>): Promise<T> {
    let attempt = 0;

    while (attempt < this.options.retries) {
      try {
        return await fn();
      } catch (error) {
        attempt++;
        if (attempt >= this.options.retries) {
          this.logger.error(`Operation failed after attempts=${attempt} error=${(error as Error).message}`);
          throw error;
        }
        this.logger.warn(
          `Attempt ${attempt} failed. Retrying in ${this.options.delayMs}ms... error=${(error as Error).message}`,
        );
        await this.delay(this.options.delayMs);
      }
    }

    throw new Error("Unexpected error in retry mechanism.");
  }

  private delay(ms: number): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }
}
