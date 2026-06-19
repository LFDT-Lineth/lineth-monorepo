/**
 * zkVM Standard Runtime C Interface
 *
 * This header defines a proposed standard C interface for guest programs to
 * terminate execution within a zkVM.
 *
 * Status: NOT yet part of the upstream zkVM standards. This interface is
 * proposed for inclusion in:
 * https://github.com/eth-act/zkvm-standards
 */

#ifndef ZKVM_STD_H
#define ZKVM_STD_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#if defined(__GNUC__) || defined(__clang__)
  #define ZKVM_NORETURN __attribute__((noreturn))
#elif defined(__cplusplus) && __cplusplus >= 201103L
  #define ZKVM_NORETURN [[noreturn]]
#elif defined(__STDC_VERSION__) && __STDC_VERSION__ >= 201112L
  #define ZKVM_NORETURN _Noreturn
#else
  #define ZKVM_NORETURN
#endif

/**
 * Halt guest execution and signal termination to the host.
 *
 * The guest stops immediately; this function never returns. The exit code is
 * reported to the host, with 0 conventionally indicating success and any
 * non-zero value indicating failure.
 *
 * @param code Exit status code reported to the host
 */
ZKVM_NORETURN void zkvm_exit(uint32_t code);

#ifdef __cplusplus
}
#endif

#endif /* ZKVM_STD_H */
