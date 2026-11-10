import{cookies}from"next/headers";import{redirect}from"next/navigation";import{AdminApiError}from"@/services/admin-api";
export async function adminCookie(){return (await cookies()).toString()}
export function adminGuard(error:unknown):never{if(error instanceof AdminApiError&&error.status===401)redirect("/admin/login");throw error}
