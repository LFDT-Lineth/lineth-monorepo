import express, { Express, Request, Response } from "express";
import { IMetricsService } from "../core/services/IMetricsService";
import { ILogger } from "../logging/ILogger";
import { IApplication } from "../core/applications/IApplication";

/**
 * Express-based API application that provides HTTP server functionality with metrics endpoint.
 * Uses protected members instead of private to allow for subclass overrides.
 * Supports optional lifecycle hooks for custom behavior during startup and shutdown.
 */
export class ExpressApiApplication implements IApplication {
  protected readonly app: Express;
  protected server?: ReturnType<Express["listen"]>;

  /**
   * Creates a new ExpressApiApplication instance.
   *
   * @param {number} port - The port number on which the server will listen.
   * @param {IMetricsService} metricsService - The metrics service for exposing Prometheus metrics.
   * @param {ILogger} logger - The logger instance for logging application events.
   */
  constructor(
    protected readonly port: number,
    protected readonly metricsService: IMetricsService,
    protected readonly logger: ILogger,
  ) {
    this.app = express();

    this.setupMiddleware();
    this.setupMetricsRoute();
  }

  /**
   * Sets up Express middleware.
   * Currently configures JSON body parsing middleware.
   * Can be overridden in subclasses to add additional middleware.
   */
  protected setupMiddleware(): void {
    this.app.use(express.json());
  }

  /**
   * Sets up the /metrics route for Prometheus metrics collection.
   * Returns metrics from the metrics service registry.
   * Handles errors gracefully by logging and returning a 500 status.
   */
  protected setupMetricsRoute() {
    this.app.get("/metrics", async (_req: Request, res: Response) => {
      try {
        const registry = this.metricsService.getRegistry();
        res.set("Content-Type", registry.contentType);
        res.end(await registry.metrics());
      } catch (error) {
        this.logger.warn("Failed to collect metrics", { error });
        res.status(500).json({ error: "Failed to collect metrics" });
      }
    });
  }

  /**
   * Starts the Express server and begins listening on the configured port.
   * Executes optional lifecycle hooks before and after starting the server.
   * Waits for the server to be ready before resolving.
   *
   * @returns {Promise<void>} A promise that resolves when the server is listening.
   */
  public async start(): Promise<void> {
    await this.onBeforeStart?.();
    this.server = this.app.listen(this.port);

    await new Promise<void>((resolve) => {
      this.server?.on("listening", () => {
        this.logger.info(`Listening on port ${this.port}`);
        resolve();
      });
    });
    await this.onAfterStart?.();
  }

  /**
   * Stops the Express server gracefully.
   * Executes optional lifecycle hooks before and after stopping the server.
   * If the server is not running, returns immediately.
   *
   * @returns {Promise<void>} A promise that resolves when the server is closed.
   * @throws {Error} If an error occurs while closing the server.
   */
  public async stop(): Promise<void> {
    await this.onBeforeStop?.();
    if (!this.server) return;

    await new Promise<void>((resolve, reject) => {
      this.server?.close((err) => {
        if (err) return reject(err);
        this.logger.info(`Closing API server on port ${this.port}`);
        this.server = undefined;
        resolve();
      });
    });
    await this.onAfterStop?.();
  }

  /**
   * Optional hook executed before the server starts listening.
   * Can be assigned from outside to perform setup operations.
   */
  onBeforeStart?: () => Promise<void> | void;
  /**
   * Optional hook executed after the server has started listening.
   * Can be assigned from outside to perform post-startup operations.
   */
  onAfterStart?: () => Promise<void> | void;
  /**
   * Optional hook executed before the server stops.
   * Can be assigned from outside to perform pre-shutdown operations.
   */
  onBeforeStop?: () => Promise<void> | void;
  /**
   * Optional hook executed after the server has stopped.
   * Can be assigned from outside to perform cleanup operations.
   */
  onAfterStop?: () => Promise<void> | void;
}
