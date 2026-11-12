import { NextRequest, NextResponse } from "next/server";

function hostname(value: string | null) {
  return (value ?? "").split(":", 1)[0].replace(/\.$/, "").toLowerCase();
}

export function surfaceAllowed(requestHost: string, pathname: string) {
  const host = hostname(requestHost);
  const publicHost = hostname(process.env.PUBLIC_HOSTNAME ?? null);
  const adminHost = hostname(process.env.ADMIN_HOSTNAME ?? null);

  if (publicHost && host === publicHost) {
    return pathname !== "/admin" && !pathname.startsWith("/admin/");
  }
  if (adminHost && host === adminHost) {
    return pathname === "/admin" || pathname.startsWith("/admin/") ||
      pathname.startsWith("/_next/") || pathname.startsWith("/images/") ||
      pathname === "/favicon.ico";
  }
  return true;
}

// Defense in depth only. The access gateway remains the security boundary.
export function proxy(request: NextRequest) {
  if (!surfaceAllowed(request.headers.get("host") ?? "", request.nextUrl.pathname)) {
    return new NextResponse(null, { status: 404 });
  }
  return NextResponse.next();
}

export const config = { matcher: "/:path*" };
