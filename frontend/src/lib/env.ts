export function getApiUrl() {
  const value = typeof window === "undefined"
    ? process.env.API_URL ?? process.env.NEXT_PUBLIC_API_URL
    : process.env.NEXT_PUBLIC_API_URL ?? window.location.origin;
  if (!value) throw new Error("API_URL ou NEXT_PUBLIC_API_URL não está configurada");
  return value.replace(/\/$/, "");
}

export function getSiteUrl() {
  return (process.env.SITE_URL ?? process.env.NEXT_PUBLIC_SITE_URL ?? "http://localhost:3000").replace(/\/$/, "");
}

