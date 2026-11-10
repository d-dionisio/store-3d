import {getApiUrl} from "@/lib/env";
export class CustomerApiError extends Error{constructor(message:string,public status:number){super(message)}}
async function call<T>(path:string,init:RequestInit={}){const response=await fetch(`${getApiUrl()}/api/v1/customer${path}`,{...init,credentials:"include",cache:"no-store",headers:{Accept:"application/json","Content-Type":"application/json",...init.headers}});if(response.status===204)return undefined as T;const body=await response.json().catch(()=>({})) as {data?:T;error?:{message?:string}};if(!response.ok||body.data===undefined)throw new CustomerApiError(body.error?.message||"Não foi possível concluir.",response.status);return body.data}
export type CustomerQuote={Reference:string;Status:string;Material:string;Quantity:number;CreatedAt:string};
export type CustomerProposal={ID:string;Status:string;Version:number;FinalPriceCents:number;ProductionLeadTimeDays:number;ValidUntil:string;CommercialNotes:string;ShippingIncluded:boolean;Decision:string;DecidedAt:string|null};
export type CustomerDetail=CustomerQuote&{Color:string;Notes:string;Proposals:CustomerProposal[]};
export async function requestMagicLink(email:string){return call<{message:string}>("/magic-links",{method:"POST",body:JSON.stringify({email})})}
export async function verifyMagicLink(token:string){return call<{csrf_token:string;user:{email:string;name:string}}>("/magic-links/verify",{method:"POST",body:JSON.stringify({token})})}
export async function profile(cookie?:string){return call<{ID:string;Email:string;FullName:string;Phone:string}>("/profile",{headers:cookie?{Cookie:cookie}:undefined})}
export async function customerQuotes(cookie?:string){return call<CustomerQuote[]>("/quotes",{headers:cookie?{Cookie:cookie}:undefined})}
export async function customerQuote(reference:string,cookie?:string){return call<CustomerDetail>(`/quotes/${encodeURIComponent(reference)}`,{headers:cookie?{Cookie:cookie}:undefined})}
export async function decide(reference:string,proposalId:string,decision:string){const csrf=sessionStorage.getItem("loja3d_customer_csrf")||"";return call<void>(`/quotes/${encodeURIComponent(reference)}/decision`,{method:"POST",headers:{"X-CSRF-Token":csrf},body:JSON.stringify({proposal_id:proposalId,decision})})}
