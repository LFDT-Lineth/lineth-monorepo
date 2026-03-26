declare global {
  namespace NodeJS {
    interface ProcessEnv {
      NEXT_PUBLIC_INFURA_ID: string;
      NEXT_PUBLIC_ALCHEMY_API_KEY: string;
      NEXT_PUBLIC_BASE_PATH: string;
    }
  }
}

export {};
