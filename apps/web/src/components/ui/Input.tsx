import React, { type InputHTMLAttributes, forwardRef } from 'react';

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ className = '', label, error, id, ...props }, ref) => {
    const inputId = id || label?.toLowerCase().replace(/\s+/g, '-');
    
    return (
      <div className="w-full">
        {label && (
          <label htmlFor={inputId} className="block text-xs font-semibold text-[#365d48] uppercase tracking-wider mb-2 ml-1">
            {label}
          </label>
        )}
        <input
          id={inputId}
          ref={ref}
          className={`block w-full rounded-2xl border-[#D6D3CD] shadow-sm transition-all duration-300 focus:border-primary-600 focus:ring-primary-600 focus:bg-white sm:text-sm px-4 py-3 bg-white/40 border outline-none backdrop-blur-sm
            ${error ? 'border-red-500 focus:border-red-500 focus:ring-red-500' : 'hover:border-[#C5C2BC]'}
            ${className}`}
          {...props}
        />
        {error && <p className="mt-1.5 ml-1 text-xs text-red-500 font-medium">{error}</p>}
      </div>
    );
  }
);
Input.displayName = 'Input';
