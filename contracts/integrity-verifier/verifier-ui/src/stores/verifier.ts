"use client";

import { create } from "zustand";
import { persist } from "zustand/middleware";
import type {
  AdapterType,
  ParsedConfig,
  FileRef,
  VerificationOptions,
  UploadedFile,
  VerifyStatus,
  FormField,
} from "@/types";
import type { VerificationSummary } from "@consensys/linea-contract-integrity-verifier";
import { getVerifierService, VerifierServiceError } from "@/services";
import { generateFormFields } from "@/lib/config-parser";
import { defaultVerificationOptions } from "@/types";

// ============================================================================
// State Interface
// ============================================================================

interface VerifierState {
  // Session
  sessionId: string | null;
  sessionLoading: boolean;
  sessionError: string | null;

  // Adapter selection
  adapter: AdapterType;

  // Config
  configFile: { name: string; size: number } | null;
  parsedConfig: ParsedConfig | null;
  configLoading: boolean;
  configError: string | null;

  // Required files
  requiredFiles: FileRef[];
  uploadedFiles: Map<string, UploadedFile>;
  fileUploadErrors: Map<string, string>;

  // Environment variables
  envVars: string[];
  envVarValues: Record<string, string>;
  envVarFields: FormField[];

  // Verification options
  options: VerificationOptions;

  // Verification state
  verifyStatus: VerifyStatus;
  verifyError: string | null;
  results: VerificationSummary | null;

  // Service readiness
  serviceReady: boolean;
}

// ============================================================================
// Actions Interface
// ============================================================================

interface VerifierActions {
  // Session
  initSession: () => Promise<void>;
  restoreSession: (sessionId: string) => Promise<void>;

  // Adapter
  setAdapter: (adapter: AdapterType) => void;

  // Config
  uploadConfig: (file: File) => Promise<void>;
  clearConfig: () => void;

  // Files
  uploadFile: (originalPath: string, file: File) => Promise<void>;
  replaceFile: (originalPath: string, file: File) => Promise<void>;
  markFileUploaded: (originalPath: string, uploadedPath: string) => void;

  // Env vars
  setEnvVar: (key: string, value: string) => void;

  // Options
  setOption: <K extends keyof VerificationOptions>(key: K, value: VerificationOptions[K]) => void;

  // Verification
  runVerification: () => Promise<void>;
  clearResults: () => void;

  // Reset
  reset: () => Promise<void>;

  // Computed
  isReadyToVerify: () => boolean;
}

// ============================================================================
// Initial State
// ============================================================================

const initialState: VerifierState = {
  sessionId: null,
  sessionLoading: false,
  sessionError: null,
  adapter: "viem",
  configFile: null,
  parsedConfig: null,
  configLoading: false,
  configError: null,
  requiredFiles: [],
  uploadedFiles: new Map(),
  fileUploadErrors: new Map(),
  envVars: [],
  envVarValues: {},
  envVarFields: [],
  options: defaultVerificationOptions,
  verifyStatus: "idle",
  verifyError: null,
  results: null,
  serviceReady: false,
};

// ============================================================================
// Store
// ============================================================================

export const useVerifierStore = create<VerifierState & VerifierActions>()(
  persist(
    (set, get) => ({
      ...initialState,

      // Session management
      initSession: async () => {
        set({ sessionLoading: true, sessionError: null });
        try {
          const service = await getVerifierService();
          set({ serviceReady: true });

          const sessionId = await service.createSession();
          set({ sessionId, sessionLoading: false });
        } catch (error) {
          const message = error instanceof VerifierServiceError ? error.message : "Failed to create session";
          set({ sessionError: message, sessionLoading: false });
        }
      },

      restoreSession: async (sessionId: string) => {
        set({ sessionLoading: true, sessionError: null });
        try {
          const service = await getVerifierService();
          set({ serviceReady: true });

          const session = await service.getSession(sessionId);
          if (session) {
            // Restore session state
            set({
              sessionId,
              sessionLoading: false,
              parsedConfig: session.config?.parsed ?? null,
              configFile: session.config ? { name: session.config.filename, size: 0 } : null,
              requiredFiles: session.config?.parsed.requiredFiles ?? [],
              envVars: session.config?.parsed.envVars ?? [],
              envVarValues: session.envVarValues,
              envVarFields: session.config?.parsed.envVars ? generateFormFields(session.config.parsed.envVars) : [],
              uploadedFiles: new Map(
                Object.entries(session.files).map(([path, file]) => [
                  path,
                  {
                    originalPath: path,
                    uploadedPath: path,
                    filename: file.filename,
                    size: file.size,
                    status: "success" as const,
                  },
                ]),
              ),
            });
          } else {
            // Session not found, create new one
            await get().initSession();
          }
        } catch {
          // Session expired or error, create new one
          await get().initSession();
        }
      },

      // Adapter selection
      setAdapter: (adapter) => {
        set({ adapter, options: { ...get().options, adapter } });
      },

      // Config upload
      uploadConfig: async (file) => {
        const { sessionId } = get();
        if (!sessionId) {
          set({ configError: "No session" });
          return;
        }

        set({
          configLoading: true,
          configError: null,
          configFile: { name: file.name, size: file.size },
        });

        try {
          const service = await getVerifierService();
          const parsedConfig = await service.saveConfig(sessionId, file);

          const fields = generateFormFields(parsedConfig.envVars);

          set({
            parsedConfig,
            requiredFiles: parsedConfig.requiredFiles,
            envVars: parsedConfig.envVars,
            envVarFields: fields,
            envVarValues: {},
            uploadedFiles: new Map(),
            fileUploadErrors: new Map(),
            configLoading: false,
          });
        } catch (error) {
          const message = error instanceof VerifierServiceError ? error.message : "Failed to upload config";
          set({
            configError: message,
            configLoading: false,
            configFile: null,
          });
        }
      },

      clearConfig: () => {
        set({
          configFile: null,
          parsedConfig: null,
          configError: null,
          requiredFiles: [],
          uploadedFiles: new Map(),
          fileUploadErrors: new Map(),
          envVars: [],
          envVarValues: {},
          envVarFields: [],
          results: null,
          verifyError: null,
          verifyStatus: "idle",
        });
      },

      // File upload
      uploadFile: async (originalPath, file) => {
        const { sessionId, requiredFiles } = get();
        if (!sessionId) return;

        const fileRef = requiredFiles.find((f) => f.path === originalPath);
        if (!fileRef) return;

        // Clear previous error
        const errors = new Map(get().fileUploadErrors);
        errors.delete(originalPath);
        set({ fileUploadErrors: errors });

        try {
          const service = await getVerifierService();
          await service.saveFile(sessionId, file, fileRef.type, originalPath);

          // Update uploaded files map
          const uploaded = new Map(get().uploadedFiles);
          uploaded.set(originalPath, {
            originalPath,
            uploadedPath: originalPath,
            filename: file.name,
            size: file.size,
            status: "success",
          });

          // Update required files status
          const updated = get().requiredFiles.map((f) => (f.path === originalPath ? { ...f, uploaded: true } : f));

          set({ uploadedFiles: uploaded, requiredFiles: updated });
        } catch (error) {
          const message = error instanceof VerifierServiceError ? error.message : "Failed to upload file";

          const errors = new Map(get().fileUploadErrors);
          errors.set(originalPath, message);
          set({ fileUploadErrors: errors });
        }
      },

      // Replace an already uploaded file
      replaceFile: async (originalPath, file) => {
        const { sessionId, requiredFiles } = get();
        if (!sessionId) return;

        const fileRef = requiredFiles.find((f) => f.path === originalPath);
        if (!fileRef) return;

        // Clear previous error
        const errors = new Map(get().fileUploadErrors);
        errors.delete(originalPath);
        set({ fileUploadErrors: errors });

        // Clear previous results since file is changing
        set({ results: null, verifyError: null, verifyStatus: "idle" });

        try {
          const service = await getVerifierService();
          await service.saveFile(sessionId, file, fileRef.type, originalPath);

          // Update uploaded files map with new file info
          const uploaded = new Map(get().uploadedFiles);
          uploaded.set(originalPath, {
            originalPath,
            uploadedPath: originalPath,
            filename: file.name,
            size: file.size,
            status: "success",
          });

          set({ uploadedFiles: uploaded });
        } catch (error) {
          const message = error instanceof VerifierServiceError ? error.message : "Failed to replace file";

          const errors = new Map(get().fileUploadErrors);
          errors.set(originalPath, message);
          set({ fileUploadErrors: errors });
        }
      },

      markFileUploaded: (originalPath, uploadedPath) => {
        const uploaded = new Map(get().uploadedFiles);
        uploaded.set(originalPath, {
          originalPath,
          uploadedPath,
          filename: originalPath.split("/").pop() || "",
          size: 0,
          status: "success",
        });

        const updated = get().requiredFiles.map((f) => (f.path === originalPath ? { ...f, uploaded: true } : f));

        set({ uploadedFiles: uploaded, requiredFiles: updated });
      },

      // Environment variables
      setEnvVar: (key, value) => {
        set({ envVarValues: { ...get().envVarValues, [key]: value } });
      },

      // Options
      setOption: (key, value) => {
        set({ options: { ...get().options, [key]: value } });
      },

      // Verification
      runVerification: async () => {
        const { sessionId, adapter, envVarValues, options } = get();
        if (!sessionId) {
          set({ verifyError: "No session" });
          return;
        }

        set({ verifyStatus: "running", verifyError: null, results: null });

        try {
          const service = await getVerifierService();
          const summary = await service.runVerification(sessionId, adapter, envVarValues, options);

          set({ results: summary, verifyStatus: "complete" });
        } catch (error) {
          const message = error instanceof VerifierServiceError ? error.message : "Verification failed";
          set({ verifyError: message, verifyStatus: "error" });
        }
      },

      clearResults: () => {
        set({ results: null, verifyError: null, verifyStatus: "idle" });
      },

      // Reset
      reset: async () => {
        set({ ...initialState });
        // Create a new session after reset
        await get().initSession();
      },

      // Computed
      isReadyToVerify: () => {
        const { parsedConfig, requiredFiles, envVars, envVarValues } = get();

        if (!parsedConfig) return false;

        // Check all required files are uploaded
        const allFilesUploaded = requiredFiles.every((f) => f.uploaded);
        if (!allFilesUploaded) return false;

        // Check all env vars have values
        const allEnvVarsSet = envVars.every((v) => envVarValues[v] && envVarValues[v].trim() !== "");
        if (!allEnvVarsSet) return false;

        return true;
      },
    }),
    {
      name: "verifier-ui-session",
      partialize: (state) => ({
        sessionId: state.sessionId,
        adapter: state.adapter,
      }),
    },
  ),
);
