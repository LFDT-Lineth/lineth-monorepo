import { cn } from "@/utils/cn";

type StepperProps = {
  steps: string[];
  activeStep: number;
};

export default function Stepper({ steps, activeStep }: StepperProps) {
  return (
    <div className="flex items-end">
      {steps.map((step, index) => (
        <div
          key={`step-${index}`}
          className={cn({
            "w-full": index !== steps.length - 1,
          })}
        >
          <div className="flex items-center">
            <div
              className={cn(
                "-mx-px flex size-14 shrink-0 items-center justify-center rounded-full border-2 border-card bg-cardBg p-1.5",
                {
                  "bg-primary": index < activeStep,
                  "border-primary": index <= activeStep,
                },
              )}
            >
              <span
                className={cn("text-base font-bold text-card", {
                  "text-cardBg": index < activeStep,
                  "text-primary": index === activeStep,
                })}
              >
                {index + 1}
              </span>
            </div>
            {index !== steps.length - 1 && (
              <div
                className={cn("h-1 w-full border-t-4 border-dotted border-card", {
                  "border-primary": index < activeStep,
                })}
              />
            )}
          </div>
          <h6 className="text-md mt-2 w-max">{step}</h6>
        </div>
      ))}
    </div>
  );
}
