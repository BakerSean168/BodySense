/**
 * Input —— 用 forwardRef 转发 DOM ref 的受控输入框封装。
 *
 * ===== 值得学的点 =====
 * - `forwardRef<HTMLInputElement, InputProps>` 把原生 DOM 节点 ref 透传给内部
 *   `<input>`，同时保持组件可被父级用 ref 直接操作底层元素。
 * - `InputProps extends InputHTMLAttributes<HTMLInputElement>`：复用原生 input 的
 *   全部合法属性，再叠加 label / error 等自有 props，剩余属性用 `{ ...props }` 透传。
 *
 * 深入笔记（Thought Forest 文件名）：
 * - react-typescript-component-and-hook-types.md
 * - react-typed-wrapper-and-polymorphic-components.md
 */

import React, { type InputHTMLAttributes, forwardRef } from "react";

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ className = "", label, error, id, ...props }, ref) => {
    const inputId = id || label?.toLowerCase().replace(/\s+/g, "-");

    return (
      <div className="w-full">
        {label && (
          <label
            htmlFor={inputId}
            className="block text-xs font-semibold text-[#365d48] uppercase tracking-wider mb-2 ml-1"
          >
            {label}
          </label>
        )}
        <input
          id={inputId}
          ref={ref}
          className={`block w-full rounded-2xl border-[#D6D3CD] shadow-sm transition-all duration-300 focus:border-primary-600 focus:ring-primary-600 focus:bg-white sm:text-sm px-4 py-3 bg-white/40 border outline-none backdrop-blur-sm
            ${error ? "border-red-500 focus:border-red-500 focus:ring-red-500" : "hover:border-[#C5C2BC]"}
            ${className}`}
          {...props}
        />
        {error && (
          <p className="mt-1.5 ml-1 text-xs text-red-500 font-medium">
            {error}
          </p>
        )}
      </div>
    );
  },
);
Input.displayName = "Input";
