import { useEffect, useState } from "react";
import { isAddress } from "viem";
import { useFormContext } from "react-hook-form";
import { MdInfo, MdAdd } from "react-icons/md";
import { Tooltip } from "@/components/ui/";

export function Recipient() {
  const [isChecked, setIsChecked] = useState(false);

  const { register, formState, setValue, setError, clearErrors, watch } = useFormContext();
  const { errors } = formState;
  const watchRecipient = watch("recipient");

  useEffect(() => {
    if (watchRecipient && !isAddress(watchRecipient)) {
      setError("recipient", {
        type: "custom",
        message: "Invalid address",
      });
    } else {
      clearErrors("recipient");
    }
  }, [watchRecipient, setError, clearErrors]);

  const toggleCheckbox = () => {
    setIsChecked(!isChecked);
    clearErrors("recipient");
    setValue("recipient", "");
  };

  return (
    <div className="collapse rounded-none">
      <input type="checkbox" className="min-h-0" onChange={toggleCheckbox} />
      <div className="collapse-title flex h-6 min-h-1 flex-row items-center gap-2 p-0 text-sm">
        <MdAdd className="size-6 text-secondary" />
        <span className="">To different address</span>
        <Tooltip
          text="Input the address you want to bridge assets to on the recipient chain"
          className="z-[99]"
          position="bottom"
        >
          <MdInfo className="text-icon" />
        </Tooltip>
      </div>

      <div className="collapse-content p-0 !pb-1 pt-2">
        <div className="form-control w-full">
          <div className="flex flex-row">
            <input
              type="text"
              className="input w-full bg-backgroundColor focus:border-none focus:outline-none"
              placeholder="0x..."
              {...register("recipient", {
                validate: (value) => !value || isAddress(value) || "Invalid address",
              })}
              maxLength={42}
            />
          </div>
          {errors.recipient && <div className="pt-2 text-error">{errors.recipient.message?.toString()}</div>}
        </div>
      </div>
    </div>
  );
}
