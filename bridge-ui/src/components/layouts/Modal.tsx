"use client";

import { useContext } from "react";
import { ModalContext } from "@/contexts/modal.context";
import { MdOutlineClose } from "react-icons/md";
import { cn } from "@/utils/cn";

type ModalProps = Record<string, never>;

const Modal: React.FC<ModalProps> = () => {
  const { ref, modalContent, options } = useContext(ModalContext);

  const width = options?.width ? options.width : "w-screen md:w-[600px]";

  return (
    <dialog ref={ref} className="modal" onClose={options?.onClose}>
      <div className={cn("modal-box px-0 bg-cardBg", width)}>
        {options?.showCloseButton && (
          <form method="dialog">
            <button className="btn btn-circle btn-ghost btn-sm absolute right-2 top-2">
              <MdOutlineClose className="text-lg" />
            </button>
          </form>
        )}
        {modalContent}
      </div>
      <form method="dialog" className="modal-backdrop">
        <button>Close</button>
      </form>
    </dialog>
  );
};

export default Modal;
