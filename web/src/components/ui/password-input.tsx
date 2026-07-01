import * as React from "react";
import { Eye, EyeOff } from "lucide-react";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

// PasswordInput wraps Input with an in-field show/hide toggle. The toggle
// owns the `type` attribute, so it is excluded from the forwarded props.
// tabIndex={-1} keeps the eye button out of the form's tab order — Tab goes
// straight from the password field to the submit button.
export const PasswordInput = React.forwardRef<
  HTMLInputElement,
  Omit<React.InputHTMLAttributes<HTMLInputElement>, "type">
>(({ className, ...props }, ref) => {
  const [showPassword, setShowPassword] = React.useState(false);
  return (
    <div className="relative">
      <Input
        ref={ref}
        className={cn("pr-10", className)}
        {...props}
        type={showPassword ? "text" : "password"}
      />
      <button
        type="button"
        tabIndex={-1}
        aria-label={showPassword ? "Hide password" : "Show password"}
        onClick={() => setShowPassword((v) => !v)}
        className="absolute right-1.5 top-1/2 -translate-y-1/2 rounded p-1 text-muted hover:text-fg"
      >
        {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
      </button>
    </div>
  );
});
PasswordInput.displayName = "PasswordInput";
