from scipy.optimize import bisect
import math

# Returns the cost of the combinatorial method described by RM08
# https://cims.nyu.edu/~regev/papers/pqc.pdf
#
# BGLS14 references 
# https://eprint.iacr.org/2014/593.pdf
#
# Depending on the value of `n`, we either do an exhaustive search
# for `n < 60` otherwise we apply a "pessimistic" algorithm, the
# attack will not bring any advantage anyway.
#
# log2_q: log2 of the modulus
# n: output size
# log2_beta: log2 of the norm bound
# m_0: input size
def cpw_bit_security(log2_q, n, log2_beta, m_0, verbose=False):
    if n < -1:
        # Runs an exhaustive search
        sec = math.inf
        min_ls = []
        for p in partitions(n, 1):
            ls = list(p)
            cpw_cost = estimate_cpw_with_parameters(log2_q, ls, log2_beta, m_0)
            hnf_cost = estimate_cpw_with_hnf_and_parameters(log2_q, ls, log2_beta, m_0)
            new_sec = min(cpw_cost, hnf_cost)
            # print("ls = {} ; sec = {}".format(ls, new_sec))
            if sec > new_sec:
                sec = new_sec
                min_ls = ls
            if verbose:
                print("ls {} - sec {}", ls, new_sec)
        if verbose:
            print("optima found at ls = {}".format(min_ls))
        return sec
    else:
        # Returns the pessimistic estimate, the 
        # instance can't be broken anyway
        return cpw_bit_security_pessimistic(log2_q, n, log2_beta, m_0)

# Returns the cost of the combinatorial method described by RM08
# https://cims.nyu.edu/~regev/papers/pqc.pdf
#
# BGLS14 references 
# https://eprint.iacr.org/2014/593.pdf
#
# As a conservative heuristic, we artificially lower the bit security estimate
# against this types of attacks by 20 bits. The function assumes that `q` is prime
# thus, the maximal value of `k` is `min(log2(m), n)`.
#
# m: input size
# n: output size
# log2_q: log2 of the modulus
# log2_beta: log2 of the norm bound
def cpw_bit_security_pessimistic(log2_q, n, log2_beta, m_0):
        
    # Density of solutions
    rhs = (m_0 * log2_beta) / (n * log2_q)

    if rhs < 1:
        # m_0 is too small to guarantee the existence of at least one solution
        return math.inf

    # Find the optimal k : we can use bisection here as `f` is a convex function
    f = lambda k : (2 ** k) / (k + 1) - rhs

    max_k = min(math.log2(m_0), n)

    # Initialize `k`
    k = None

    # Early test, if even the largest value of `k` 
    # does not allow for a solution, return `inf`
    if f(max_k) <= 0:
        # Instance is unbreakable using the CPW attack
        k = max_k
    elif f(0) >= 0:
        # Instance can only be attacked with direct birthday attack? This may 
        # happen if the density of solutions is 1. In which case, splitting the 
        # input domain is not possible.
        k = 0
    else:
        k = bisect(f, 0, max_k)
        k = math.floor(k)
         
    # Reduce m, as much as it can help
    f = lambda m : (m * log2_beta) / (n * log2_q) - (2 ** k) / (k + 1)
    # 
    m = m_0
    if f(0) * f(m_0) < 0:
        m = bisect(f, 0, m_0)
        m = math.ceil(m)
        
    return k + log2_beta * (m / (2**k))

# Estimate HNF security with parameters
def estimate_cpw_with_hnf_and_parameters(log2_q, ls, log2_beta, m_0):
    #
    # CPW depth tree
    k = len(ls)
    nb_chunks = 2**(k-1)
    n = sum(ls)
    # 
    # Compute the optimal value of `m`
    optimal_m = log2_q / log2_beta * sum([ls[i] * 2**(k-i-1) for i in range(k)])
    # The -n accounts for the fact that with HNF
    # there is a `n` parameters that we can no 
    # longer consider `free`
    m = min(m_0-n, optimal_m)
    chunk_size = m / nb_chunks
    # print("chunksize = {}, k = {}, nb_chunks = {}".format(chunk_size, k, nb_chunks))
    #
    cur_log_list_size = chunk_size * log2_beta - (log2_q * ls[0])
    log_runtime = chunk_size * log2_beta * .5 + k

    # print("cur_log_list_size {}".format(cur_log_list_size))

    for i in range(1, k):
        gamma_i = ls[i] * 2**(k-i)
        new_log_list_size = 2*cur_log_list_size - (log2_q * ls[i]) + gamma_i * log2_beta
        # print("i = {} | cur_log_list_size = {} | new_log_list_size = {}".format(i, cur_log_list_size, new_log_list_size))
        # Time elapsed during the round
        elapsed_time = new_log_list_size + k - i
        cur_log_list_size = new_log_list_size
        # print("cur_log_list_size {}".format(cur_log_list_size))
        # Record the running time
        if max(log_runtime, elapsed_time) > 1000:
            # In that case, simply take the maximal value. The instance
            # is out of reach. No need to be accurate
            log_runtime = max(log_runtime, new_log_list_size + k - i)
        else:
            log_runtime = math.log2(2**log_runtime + 2**elapsed_time)

        # If the final list has only small probability to contain 
        # at least one element, then we need to repeat.
        if cur_log_list_size < 0:
            log_runtime -= cur_log_list_size
    return log_runtime

# Return the runtime of a parametrized CPW attack
def estimate_cpw_with_parameters(log2_q, ls, log2_beta, m_0):
    #
    # CPW depth tree
    k = len(ls)
    nb_chunks = 2**(k-1)
    # 
    # Compute the optimal value of `m`
    optimal_m = log2_q / log2_beta * sum([ls[i] * 2**(k-i-1) for i in range(k)])
    m = min(m_0, optimal_m)
    chunk_size = m / nb_chunks
    # print("chunksize = {}, k = {}, nb_chunks = {}, optimal m = {}".format(chunk_size, k, nb_chunks, m))
    #
    cur_log_list_size = chunk_size * log2_beta - (log2_q * ls[0])
    log_runtime = chunk_size * log2_beta * .5 + k

    # print("cur_log_list_size {}".format(cur_log_list_size))

    for i in range(1, k):
        new_log_list_size = 2*cur_log_list_size - (log2_q * ls[i])
        # print("i = {} | cur_log_list_size = {} | new_log_list_size = {}".format(i, cur_log_list_size, new_log_list_size))
        # Time elapsed during the round
        elapsed_time = new_log_list_size + k - i
        cur_log_list_size = new_log_list_size
        # print("cur_log_list_size {}".format(cur_log_list_size))
        # Record the running time
        if max(log_runtime, elapsed_time) > 1000:
            # In that case, simply take the maximal value. The instance
            # is out of reach. No need to be accurate
            log_runtime = max(log_runtime, new_log_list_size + k - i)
        else:
            log_runtime = math.log2(2**log_runtime + 2**elapsed_time)

        # If the final list has only small probability to contain 
        # at least one element, then we need to repeat.
        if cur_log_list_size < 0:
            log_runtime -= cur_log_list_size
    return log_runtime

# Copy pasted from there :
# https://stackoverflow.com/questions/10035752/elegant-python-code-for-integer-partitioning
def partitions(n, I=1):
    yield (n,)
    for i in range(I, n//2 + 1):
        for p in partitions(n-i, i):
            yield (i,) + p
