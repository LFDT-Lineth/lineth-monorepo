package linea.crypto

import java.util.concurrent.atomic.AtomicBoolean

/** A signer whose owning component must release its backing resources. */
interface CloseableSigner<T> :
  Signer<T>,
  AutoCloseable

/** Couples this signer to an idempotent resource cleanup action. */
fun <T> Signer<T>.withCloseAction(closeAction: () -> Unit = {}): CloseableSigner<T> =
  object : CloseableSigner<T> {
    private val closed = AtomicBoolean()

    override fun publicKey(): ByteArray = this@withCloseAction.publicKey()

    override fun sign(bytes: ByteArray) = this@withCloseAction.sign(bytes)

    override fun close() {
      if (closed.compareAndSet(false, true)) {
        closeAction()
      }
    }
  }
