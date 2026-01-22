import * as React from "react"
import { cn } from "@/lib/utils"

interface ScrollAreaProps extends React.HTMLAttributes<HTMLDivElement> {
  orientation?: "vertical" | "horizontal"
}

const ScrollArea = React.forwardRef<HTMLDivElement, ScrollAreaProps>(
  ({ className, children, orientation = "vertical", ...props }, ref) => {
    return (
      <div
        ref={ref}
        className={cn(
          "relative overflow-hidden",
          className
        )}
        {...props}
      >
        <div
          className={cn(
            "h-full w-full",
            orientation === "vertical" ? "overflow-y-auto" : "overflow-x-auto",
            "scrollbar-thin scrollbar-thumb-border scrollbar-track-transparent"
          )}
          style={{
            scrollbarWidth: "thin",
            scrollbarColor: "hsl(var(--border)) transparent"
          }}
        >
          {children}
        </div>
      </div>
    )
  }
)
ScrollArea.displayName = "ScrollArea"

export { ScrollArea }
