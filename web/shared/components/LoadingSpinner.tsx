import { Loader2 } from "lucide-react";
import { cn } from "../lib/utils.js";

type LoadingSpinnerProps = {
  className?: string;
  size?: number;
};

export function LoadingSpinner({ className, size = 24 }: LoadingSpinnerProps) {
  return (
    <div className={cn("flex items-center justify-center p-8", className)}>
      <Loader2 className="animate-spin text-muted-foreground" size={size} />
    </div>
  );
}
