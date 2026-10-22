/**
 * Input —— shadcn (base-ui) 输入框，使用应用的 `--ui-*` 纸张主题变量。
 *
 * - `variant`: field（圆角描边，默认）/ underline（下划线）/ bare（无边框，
 *   由所在行绘制下划线）/ ghost。
 * - 兼容旧用法：传入 `label` / `error` 时会渲染标签和错误提示。
 * - ref 直接指向底层 `<input>`。
 */
import * as React from "react";
import { Input as InputPrimitive } from "@base-ui/react/input";

import { cn } from "@/lib/utils";
import {
  fieldVariants,
  type FieldVariant,
} from "@/components/ui/field-variants";

export interface InputProps extends Omit<
  React.ComponentProps<"input">,
  "size"
> {
  variant?: FieldVariant;
  label?: string;
  error?: string;
}

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, variant = "field", label, error, id, type, ...props }, ref) => {
    const generatedId = React.useId();
    const inputId =
      id || (label ? label.toLowerCase().replace(/\s+/g, "-") : undefined);
    const control = (
      <InputPrimitive
        ref={ref}
        id={inputId ?? (error ? generatedId : undefined)}
        type={type}
        data-slot="input"
        data-variant={variant}
        aria-invalid={error ? true : props["aria-invalid"]}
        className={cn(
          fieldVariants({ variant }),
          "py-1.5 file:border-0 file:bg-transparent file:text-sm file:font-medium [&[type=number]]:[appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none",
          className,
        )}
        {...props}
      />
    );
    if (!label && !error) return control;
    return (
      <div className="w-full">
        {label ? (
          <label
            htmlFor={inputId}
            className="mb-2 ml-1 block text-xs font-semibold tracking-wider text-(--ui-primary) uppercase"
          >
            {label}
          </label>
        ) : null}
        {control}
        {error ? (
          <p className="mt-1.5 ml-1 text-xs font-medium text-(--ui-danger)">
            {error}
          </p>
        ) : null}
      </div>
    );
  },
);
Input.displayName = "Input";
