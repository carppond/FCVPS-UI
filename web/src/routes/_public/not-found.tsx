import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/_public/not-found")({
  component: NotFoundPage,
});

/**
 * 404 / silent-mode placeholder page.
 * Mimics an nginx default 404 page to reduce fingerprinting.
 */
function NotFoundPage() {
  return (
    <div
      style={{
        fontFamily: "sans-serif",
        textAlign: "center",
        padding: "60px 20px",
        color: "#888",
      }}
    >
      <h1 style={{ fontSize: "6rem", margin: 0, color: "#aaa" }}>404</h1>
      <h2 style={{ margin: "0 0 16px" }}>Not Found</h2>
      <hr style={{ border: "none", borderTop: "1px solid #ddd", margin: "0 auto 16px", width: "60%" }} />
      <p style={{ fontSize: "0.8rem" }}>nginx/1.27.0</p>
    </div>
  );
}
