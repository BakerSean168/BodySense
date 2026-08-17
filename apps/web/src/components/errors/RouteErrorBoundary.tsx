import { Component, type ErrorInfo, type ReactNode } from "react";
import { AlertTriangle, RotateCcw } from "lucide-react";
import { useLocation } from "react-router";
import { Button } from "@/components/ui/Button";

interface BoundaryProps {
  children: ReactNode;
  resetKey: string;
  variant: "default" | "consultation";
}

interface BoundaryState {
  error: Error | null;
}

class RouteBoundary extends Component<BoundaryProps, BoundaryState> {
  override state: BoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): BoundaryState {
    return { error };
  }

  override componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("Route rendering failed", error, info.componentStack);
  }

  override componentDidUpdate(previous: BoundaryProps) {
    if (previous.resetKey !== this.props.resetKey && this.state.error) {
      this.setState({ error: null });
    }
  }

  override render() {
    if (!this.state.error) return this.props.children;
    return (
      <div className="flex min-h-dvh items-center justify-center bg-background p-6 text-foreground">
        <section
          role="alert"
          aria-labelledby="route-error-title"
          className="w-full max-w-lg rounded-2xl border border-destructive/20 bg-card p-6 text-center shadow-sm"
        >
          <span className="mx-auto flex size-12 items-center justify-center rounded-full bg-destructive/10 text-destructive">
            <AlertTriangle className="size-5" aria-hidden="true" />
          </span>
          <h1 id="route-error-title" className="mt-4 text-lg font-semibold">
            {this.props.variant === "consultation"
              ? "健康工作区暂时无法显示"
              : "当前页面暂时无法显示"}
          </h1>
          <p className="mt-2 text-sm leading-6 text-muted-foreground">
            已保留服务器中的长期数据。重新加载页面后可以继续；若问题持续，请稍后再试。
          </p>
          <Button
            type="button"
            className="mt-5"
            onClick={() => window.location.reload()}
          >
            <RotateCcw className="size-4" aria-hidden="true" />
            重新加载
          </Button>
        </section>
      </div>
    );
  }
}

export function RouteErrorBoundary({
  children,
  variant = "default",
}: {
  children: ReactNode;
  variant?: "default" | "consultation";
}) {
  const location = useLocation();
  return (
    <RouteBoundary
      resetKey={`${location.pathname}${location.search}`}
      variant={variant}
    >
      {children}
    </RouteBoundary>
  );
}
