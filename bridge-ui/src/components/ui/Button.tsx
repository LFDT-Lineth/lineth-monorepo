import React, { useMemo, forwardRef } from "react";
import { cn } from "@/utils/cn";

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "primary" | "secondary" | "outline" | "link";
  size?: "xs" | "sm" | "md" | "lg";
  loading?: boolean;
}

const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      type = "button",
      variant = "primary",
      size = "md",
      disabled = false,
      loading = false,
      onClick,
      children,
      className,
      id,
      ...props
    },
    ref,
  ) => {
    const computedClassName = useMemo(() => {
      const baseClasses = "btn rounded-full uppercase";
      const variantClasses = {
        primary: "btn-primary",
        secondary: "btn-secondary",
        outline: "btn-outline",
        link: "btn-link",
      }[variant];
      const sizeClasses = {
        xs: "btn-xs",
        sm: "btn-sm",
        md: "btn-md",
        lg: "btn-lg",
      }[size];

      return cn(baseClasses, variantClasses, sizeClasses, className, {
        "btn-disabled": disabled,
        "cursor-wait btn-disabled": loading,
      });
    }, [variant, size, className, disabled, loading]);

    return (
      <button
        ref={ref}
        id={id}
        type={type}
        className={computedClassName}
        onClick={onClick}
        disabled={disabled}
        {...props}
      >
        {loading && <span className="loading loading-spinner"></span>}
        {children}
      </button>
    );
  },
);

Button.displayName = "Button";

export default Button;
