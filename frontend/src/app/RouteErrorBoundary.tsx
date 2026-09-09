import { Component, type ErrorInfo, type ReactNode } from "react";
import { X } from "lucide-react";

interface RouteErrorBoundaryProps {
  children: ReactNode;
}

interface RouteErrorBoundaryState {
  failed: boolean;
}

export class RouteErrorBoundary extends Component<RouteErrorBoundaryProps, RouteErrorBoundaryState> {
  state: RouteErrorBoundaryState = { failed: false };

  static getDerivedStateFromError(): RouteErrorBoundaryState {
    return { failed: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("route chunk failed to load", error, info);
  }

  render() {
    if (!this.state.failed) return this.props.children;
    return (
      <div className="state-block error-state" role="alert">
        <X size={28} aria-hidden="true" />
        <h2>页面加载失败</h2>
        <p>页面资源可能已更新或网络暂时不可用，请重新加载后重试。</p>
        <button className="secondary-button" type="button" onClick={() => window.location.reload()}>重新加载页面</button>
      </div>
    );
  }
}
