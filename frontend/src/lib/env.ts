const apiUrl = process.env.NEXT_PUBLIC_API_URL;

if (!apiUrl) {
  throw new Error("NEXT_PUBLIC_API_URL is not configured");
}

export const env = {
  apiUrl: apiUrl.replace(/\/$/, ""),
};

