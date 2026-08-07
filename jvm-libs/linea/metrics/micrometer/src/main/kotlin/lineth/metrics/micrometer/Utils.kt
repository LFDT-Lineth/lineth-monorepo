package lineth.metrics.micrometer

fun elapsedTimeInMillisSince(startTime: Long): Long = (System.nanoTime() - startTime) / 1_000_000
