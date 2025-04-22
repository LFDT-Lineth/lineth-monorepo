import express, { Express, Request, Response } from "express";
import { IMetricsService } from "../../../core/metrics/IMetricsService";
import { ILogger } from "../../../core/utils/logging/ILogger";

type ApiConfig = {
  port: number;
};

export class Api {
  private readonly app: Express;
  private server?: ReturnType<Express["listen"]>;

  constructor(
    private readonly config: ApiConfig,
    private readonly metricsService: IMetricsService,
    private readonly logger: ILogger,
  ) {
    this.app = express();

    this.setupMiddleware();
    this.setupRoutes();
  }

  private setupMiddleware(): void {
    this.app.use(express.json());
  }

  private setupRoutes(): void {
    this.app.get("/metrics", this.handleMetrics.bind(this));
  }

  private async handleMetrics(_req: Request, res: Response): Promise<void> {
    try {
      const registry = this.metricsService.getRegistry();
      res.set("Content-Type", registry.contentType);
      res.end(await registry.metrics());
    } catch (error) {
      res.status(500).json({ error: "Failed to collect metrics" });
    }
  }

  public async start(): Promise<void> {
    this.server = this.app.listen(this.config.port);

    await new Promise<void>((resolve) => {
      this.server?.on("listening", () => {
        this.logger.info(`Listening on port ${this.config.port}`);
        resolve();
      });
    });
  }

  public async stop(): Promise<void> {
    if (!this.server) return;

    await new Promise<void>((resolve, reject) => {
      this.server?.close((err) => {
        if (err) return reject(err);
        this.logger.info(`Closing API server on port ${this.config.port}`);
        this.server = undefined;
        resolve();
      });
    });
  }
}
