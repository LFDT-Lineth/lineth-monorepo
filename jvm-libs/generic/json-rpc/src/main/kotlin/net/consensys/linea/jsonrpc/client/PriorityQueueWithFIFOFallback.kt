package net.consensys.linea.jsonrpc.client

import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import java.util.PriorityQueue
import java.util.Queue

/**
 * only used  LoadBalancingJsonRpcClient
 * so not methods implemented yet due incident urgency
 *
 */
internal class PriorityQueueWithFIFOFallback<T>(
  private val comparator: Comparator<T>,
  private val log: Logger = LogManager.getLogger(PriorityQueueWithFIFOFallback::class.java),
) : Queue<T> {
  private val priorityQueue = PriorityQueue<IndexedItem<T>>(
    Comparator { o1, o2 ->
      try {
        comparator.compare(o1.item, o2.item)
      } catch (e: Exception) {
        log.warn("failed to compare items: error={} o1={} o2={}", e.message, o1, o2)
        // Fallback to FIFO on comparison failure
        o1.insertionOrder.compareTo(o2.insertionOrder)
      }
    },
  )
  private var insertionCounter = 0L

  private data class IndexedItem<T>(
    val item: T,
    val insertionOrder: Long,
  )

  override fun add(element: T): Boolean {
    return priorityQueue.add(IndexedItem(element, insertionCounter++))
  }

  override fun poll(): T? = priorityQueue.poll()?.item

  override fun peek(): T? = priorityQueue.peek()?.item

  override fun isEmpty(): Boolean = priorityQueue.isEmpty()

  override val size: Int get() = priorityQueue.size

  fun toList(): List<T> = priorityQueue.map { it.item }

  override fun remove(element: T?): Boolean {
    throw UnsupportedOperationException("remove(element) is not supported")
  }

  override fun addAll(elements: Collection<T?>): Boolean {
    throw UnsupportedOperationException("addAll is not supported")
  }

  override fun clear() {
    priorityQueue.clear()
  }

  override fun iterator(): MutableIterator<T?> {
    throw UnsupportedOperationException("iterator is not supported")
  }

  override fun removeAll(elements: Collection<T?>): Boolean {
    throw UnsupportedOperationException("removeAll is not supported")
  }

  override fun retainAll(elements: Collection<T?>): Boolean {
    throw UnsupportedOperationException("retainAll is not supported")
  }

  override fun contains(element: T): Boolean {
    return priorityQueue.any { it.item == element }
  }

  override fun containsAll(elements: Collection<T?>): Boolean {
    throw UnsupportedOperationException("containsAll is not supported")
  }

  override fun offer(e: T): Boolean = add(e)
  override fun remove(): T = poll() ?: throw NoSuchElementException()
  override fun element(): T = peek() ?: throw NoSuchElementException()
}
