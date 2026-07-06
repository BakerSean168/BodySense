import React, { type HTMLAttributes } from 'react';

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  glass?: boolean;
}

export function Card({ className = '', glass = false, children, ...props }: CardProps) {
  return (
    <div 
      className={`rounded-[24px] transition-all duration-300 ${glass ? 'glass' : 'editorial-card'} ${className}`} 
      {...props}
    >
      {children}
    </div>
  );
}
