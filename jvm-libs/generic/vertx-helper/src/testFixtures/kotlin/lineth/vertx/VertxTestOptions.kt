package lineth.vertx

import io.vertx.core.VertxOptions

/**
 * Default [VertxOptions] for tests: small, fixed-size thread pools so that test suites
 * consume fewer resources (CPU/threads) when run in parallel, e.g. in CI.
 */
val vertxTestOptions: VertxOptions =
  VertxOptions()
    .setEventLoopPoolSize(2)
    .setWorkerPoolSize(2)
    .setInternalBlockingPoolSize(2)
