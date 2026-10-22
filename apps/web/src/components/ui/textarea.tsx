import * as React from "react";

import { cn } from "@/lib/utils";
import {
  fieldVariants,
  type FieldVariant,
} from "@/components/ui/field-variants";

function Textarea({
  className,
  variant = "field",
  ...props
}: React.ComponentProps<"textarea"> & { variant?: FieldVariant }) {
  return (
    <textarea
      data-slot="textarea"
      data-variant={variant}
      className={cn(
        fieldVariants({ variant }),
        "block min-h-20 resize-y py-2.5 leading-relaxed break-words",
        className,
      )}
      {...props}
    />
  );
}

export { Textarea };
