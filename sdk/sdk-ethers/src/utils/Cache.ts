/* eslint-disable @typescript-eslint/no-explicit-any */
import { LRUCache } from "lru-cache";
import { serialize, isUndefined } from "../core/utils";

interface CacheInterface {
  get: (key: string) => any;
  set: (key: string, value: any) => void;
}

export class Cache {
  private cache: CacheInterface;

  /**
   * Initializes a new instance of the `Cache` class.
   *
   * The constructor determines the execution environment (Node.js or browser) and initializes the appropriate caching mechanism.
   */
  constructor() {
    if (!isUndefined(process) && process.versions != null && process.versions.node != null) {
      this.cache = new LRUCache({ max: 500 });
    } else {
      this.cache = {
        get(key: string) {
          const value = localStorage.getItem(key);
          return value ? JSON.parse(value) : undefined;
        },
        set(key: string, value: any) {
          localStorage.setItem(key, serialize(value));
        },
      };
    }
  }

  /**
   * Retrieves a value from the cache by its key.
   *
   * @param {string} key - The key associated with the value to retrieve.
   * @returns {any} The value associated with the key, or `undefined` if the key does not exist in the cache.
   */
  public get(key: string): any {
    return this.cache.get(key);
  }

  /**
   * Stores a value in the cache with an associated key.
   *
   * @param {string} key - The key to associate with the value.
   * @param {any} value - The value to store in the cache.
   */
  public set(key: string, value: any) {
    this.cache.set(key, value);
  }
}
