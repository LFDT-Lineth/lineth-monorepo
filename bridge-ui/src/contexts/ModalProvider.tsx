"use client";

import React, { createContext, useCallback, useContext, useMemo, useState } from "react";

interface ModalContextType {
  isModalOpen: boolean;
  isModalType: string;
  modalData: Record<string, unknown>;
  updateModal: (open: boolean, type: string, data?: Record<string, unknown>) => void;
}

const ModalContext = createContext<ModalContextType | undefined>(undefined);

export const useModal = () => {
  const context = useContext(ModalContext);
  if (!context) {
    throw new Error("useModal must be used within a ModalProvider");
  }
  return context;
};

export const ModalProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [stateIsModalOpen, stateSetIsModalOpen] = useState<boolean>(false);
  const [stateIsModalType, stateSetIsModalType] = useState<string>("spins");
  const [stateModalData, stateSetModalData] = useState<Record<string, unknown>>({});

  const updateModal = useCallback((open: boolean, type: string, data?: Record<string, unknown>) => {
    stateSetIsModalOpen(open);
    stateSetIsModalType(type);
    stateSetModalData(data || {}); // Fallback to an empty object if no data is passed
  }, []);

  const value = useMemo(
    () => ({
      isModalOpen: stateIsModalOpen,
      isModalType: stateIsModalType,
      modalData: stateModalData,
      updateModal,
    }),
    [stateIsModalOpen, stateIsModalType, stateModalData, updateModal],
  );

  return <ModalContext.Provider value={value}>{children}</ModalContext.Provider>;
};
