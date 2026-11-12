import { getApiUrl } from "@/lib/env";
import { mapProduct, type ProductDTO } from "@/services/catalog-api";
import { AdminApiError } from "@/services/admin-api";
import type { Product } from "@/types/product";

export type EditorialCatalog = { id:string;slug:string;title:string;description:string;eyebrow:string;editionLabel:string;coverImageId:string|null;coverImageUrl:string;status:"draft"|"published";version:number;publishedAt:string|null;createdAt:string;updatedAt:string;productIds:string[];products:Product[] };
type RawCatalog={id:string;slug:string;title:string;description:string;eyebrow:string;edition_label:string;cover_image_id:string|null;cover_image_url:string;status:"draft"|"published";version:number;published_at:string|null;created_at:string;updated_at:string;product_ids:string[]|null;products:ProductDTO[]|null};
const map=(value:RawCatalog):EditorialCatalog=>({id:value.id,slug:value.slug,title:value.title,description:value.description,eyebrow:value.eyebrow,editionLabel:value.edition_label,coverImageId:value.cover_image_id,coverImageUrl:value.cover_image_url,status:value.status,version:value.version,publishedAt:value.published_at,createdAt:value.created_at,updatedAt:value.updated_at,productIds:value.product_ids??[],products:(value.products??[]).map(mapProduct)});
async function publicRequest(path:string){const response=await fetch(`${getApiUrl()}/api/v1/editorial-catalogs${path}`,{cache:"no-store",headers:{Accept:"application/json"}});if(response.status===404)return null;if(!response.ok)throw new Error("Não foi possível consultar os catálogos editoriais.");return response.json() as Promise<{data:RawCatalog|RawCatalog[]}>}
export async function getPublishedCatalogs(){const body=await publicRequest("");return body? (body.data as RawCatalog[]).map(map):[]}
export async function getPublishedCatalog(slug:string){const body=await publicRequest(`/${encodeURIComponent(slug)}`);return body?map(body.data as RawCatalog):null}

async function adminRequest<T>(path:string,init:RequestInit={},cookie?:string){const response=await fetch(`${getApiUrl()}/api/v1/admin/catalogs${path}`,{...init,credentials:"include",cache:"no-store",headers:{Accept:"application/json","Content-Type":"application/json",...(cookie?{Cookie:cookie}:{}),...init.headers}});const body=await response.json().catch(()=>({})) as {data?:T;error?:{message?:string}};if(!response.ok||body.data===undefined)throw new AdminApiError(body.error?.message??"Falha na API administrativa.",response.status);return body.data}
const csrf=()=>typeof window==="undefined"?"":sessionStorage.getItem("loja3d_admin_csrf")??"";
export async function getAdminCatalogs(query="",cookie?:string){const value=await adminRequest<{items:RawCatalog[];meta:{page:number;limit:number;total:number;total_pages:number}}>(query,{},cookie);return{items:value.items.map(map),meta:value.meta}}
export async function getAdminCatalog(id:string,cookie?:string){return map(await adminRequest<RawCatalog>(`/${encodeURIComponent(id)}`,{},cookie))}
export type EditorialInput={slug:string;title:string;description:string;eyebrow:string;edition_label:string;cover_image_id:string|null;expected_version?:number};
export async function createAdminCatalog(input:EditorialInput){return map(await adminRequest<RawCatalog>("",{method:"POST",headers:{"X-CSRF-Token":csrf()},body:JSON.stringify(input)}))}
export async function updateAdminCatalog(id:string,input:EditorialInput){return map(await adminRequest<RawCatalog>(`/${id}`,{method:"PUT",headers:{"X-CSRF-Token":csrf()},body:JSON.stringify(input)}))}
export async function replaceAdminCatalogProducts(id:string,productIds:string[],version:number){return map(await adminRequest<RawCatalog>(`/${id}/products`,{method:"PUT",headers:{"X-CSRF-Token":csrf()},body:JSON.stringify({product_ids:productIds,expected_version:version})}))}
export async function setAdminCatalogPublication(id:string,publish:boolean,version:number){return map(await adminRequest<RawCatalog>(`/${id}/${publish?"publish":"unpublish"}`,{method:"POST",headers:{"X-CSRF-Token":csrf()},body:JSON.stringify({expected_version:version})}))}
