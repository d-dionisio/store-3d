import {getApiUrl} from "@/lib/env";
export class CustomerApiError extends Error{constructor(message:string,public status:number){super(message)}}
async function call<T>(path:string,init:RequestInit={}){const response=await fetch(`${getApiUrl()}/api/v1/customer${path}`,{...init,credentials:"include",cache:"no-store",headers:{Accept:"application/json","Content-Type":"application/json",...init.headers}});if(response.status===204)return undefined as T;const body=await response.json().catch(()=>({})) as {data?:T;error?:{message?:string}};if(!response.ok||body.data===undefined)throw new CustomerApiError(body.error?.message||"Não foi possível concluir.",response.status);return body.data}
export type CustomerQuote={Reference:string;Status:string;Material:string;Quantity:number;CreatedAt:string};
export type CustomerProposal={ID:string;Status:string;Version:number;FinalPriceCents:number;ProductionLeadTimeDays:number;ValidUntil:string;CommercialNotes:string;ShippingIncluded:boolean;Decision:string;DecidedAt:string|null};
export type CustomerDetail=CustomerQuote&{Color:string;Notes:string;Proposals:CustomerProposal[]};
export async function requestMagicLink(email:string){return call<{message:string}>("/magic-links",{method:"POST",body:JSON.stringify({email})})}
export async function verifyMagicLink(token:string){return call<{csrf_token:string;user:{email:string;name:string}}>("/magic-links/verify",{method:"POST",body:JSON.stringify({token})})}
export type AuthStep={action:"authenticated"|"mfa_required"|"mfa_setup_required";challenge_token?:string;csrf_token?:string;setup_uri?:string;setup_secret?:string;qr_code_data_url?:string;recovery_codes?:string[]};
export async function registerCustomer(full_name:string,email:string,password:string){return call<{message:string}>("/register",{method:"POST",body:JSON.stringify({full_name,email,password})})}
export async function passwordLogin(email:string,password:string){return call<AuthStep>("/session",{method:"POST",body:JSON.stringify({email,password})})}
export async function completeCustomerMFA(challenge_token:string,code:string,recovery_code=""){return call<{csrf_token:string;recovery_codes:string[]}>("/session/mfa",{method:"POST",body:JSON.stringify({challenge_token,code,recovery_code})})}
export async function verifyEmailToken(token:string){return call<void>("/email/verify",{method:"POST",body:JSON.stringify({token})})}
export async function resendVerification(email:string){return call<{message:string}>("/email/resend",{method:"POST",body:JSON.stringify({email})})}
export async function forgotPassword(email:string){return call<{message:string}>("/password/forgot",{method:"POST",body:JSON.stringify({email})})}
export async function resetPassword(token:string,password:string){return call<void>("/password/reset",{method:"POST",body:JSON.stringify({token,password})})}
export async function securityStatus(){return call<{mfa_enabled:boolean;recovery_codes_remaining:number;authentication_level:string}>("/security")}
export async function beginMFA(){const csrf=sessionStorage.getItem("loja3d_customer_csrf")||"";return call<AuthStep>("/security/mfa",{method:"POST",headers:{"X-CSRF-Token":csrf}})}
export async function disableMFA(){const csrf=sessionStorage.getItem("loja3d_customer_csrf")||"";return call<void>("/security/mfa",{method:"DELETE",headers:{"X-CSRF-Token":csrf}})}
export async function profile(cookie?:string){return call<{ID:string;Email:string;FullName:string;Phone:string}>("/profile",{headers:cookie?{Cookie:cookie}:undefined})}
export async function customerQuotes(cookie?:string){return call<CustomerQuote[]>("/quotes",{headers:cookie?{Cookie:cookie}:undefined})}
export async function customerQuote(reference:string,cookie?:string){return call<CustomerDetail>(`/quotes/${encodeURIComponent(reference)}`,{headers:cookie?{Cookie:cookie}:undefined})}
export async function decide(reference:string,proposalId:string,decision:string){const csrf=sessionStorage.getItem("loja3d_customer_csrf")||"";return call<void>(`/quotes/${encodeURIComponent(reference)}/decision`,{method:"POST",headers:{"X-CSRF-Token":csrf},body:JSON.stringify({proposal_id:proposalId,decision})})}
export async function convertQuote(reference:string){const csrf=sessionStorage.getItem("loja3d_customer_csrf")||"";return call<{reference:string;status:string}>(`/quotes/${encodeURIComponent(reference)}/convert`,{method:"POST",headers:{"X-CSRF-Token":csrf}})}
export type CartInput={variant_id:string;quantity:number};
export type CartLine={VariantID:string;ProductName:string;SKU:string;Material:string;Color:string;Quantity:number;AvailableQuantity:number;UnitPriceCents:number;LineTotalCents:number};
export type ValidatedCart={Items:CartLine[];SubtotalCents:number};
export type CustomerAddress={ID:string;RecipientName:string;PostalCode:string;Street:string;Number:string;Complement:string;District:string;City:string;StateCode:string;CountryCode:string;IsDefault:boolean};
export async function validateCart(items:CartInput[]){return call<ValidatedCart>("/cart/validate",{method:"POST",body:JSON.stringify({items})})}
export async function listAddresses(){return call<CustomerAddress[]>("/addresses")}
export async function createAddress(address:Omit<CustomerAddress,"ID">){const csrf=sessionStorage.getItem("loja3d_customer_csrf")||"";return call<CustomerAddress>("/addresses",{method:"POST",headers:{"X-CSRF-Token":csrf},body:JSON.stringify(address)})}
export async function createCatalogOrder(items:CartInput[],addressId:string,key:string){const csrf=sessionStorage.getItem("loja3d_customer_csrf")||"";return call<{reference:string;status:string}>("/orders/catalog",{method:"POST",headers:{"X-CSRF-Token":csrf,"Idempotency-Key":key},body:JSON.stringify({items,address_id:addressId})})}
export type CustomerOrder={Reference:string;Status:string;PaymentReadiness:string;TotalCents:number;CreatedAt:string};
export async function listOrders(cookie?:string){return call<CustomerOrder[]>("/orders",{headers:cookie?{Cookie:cookie}:undefined})}
export async function getOrder(reference:string,cookie?:string){return call<CustomerOrder&{Items:CartLine[];Address:CustomerAddress|null;ShippingRequired:boolean}>(`/orders/${encodeURIComponent(reference)}`,{headers:cookie?{Cookie:cookie}:undefined})}
export type ShippingQuote={ID:string;ServiceCode:string;CarrierName:string;ServiceName:string;PriceCents:number;MinDays:number;MaxDays:number;ExpiresAt:string};
export async function quoteShipping(reference:string){const csrf=sessionStorage.getItem("loja3d_customer_csrf")||"";return call<ShippingQuote[]>(`/orders/${encodeURIComponent(reference)}/shipping-quotes`,{method:"POST",headers:{"X-CSRF-Token":csrf}})}
export async function selectShipping(reference:string,quoteId:string){const csrf=sessionStorage.getItem("loja3d_customer_csrf")||"";return call<void>(`/orders/${encodeURIComponent(reference)}/shipping-selection`,{method:"POST",headers:{"X-CSRF-Token":csrf},body:JSON.stringify({quote_id:quoteId})})}
export type PixPayment={AttemptID:string;Status:string;QRCode:string;QRCodeBase64:string;TicketURL:string;AmountCents:number;ExpiresAt:string|null};
export type PaymentStatus={AttemptID:string;OrderReference:string;Method:string;Status:string;AmountCents:number;PaidAmountCents:number;RefundedAmountCents:number;ExpiresAt:string|null;PaidAt:string|null};
export async function createPix(reference:string){const csrf=sessionStorage.getItem("loja3d_customer_csrf")||"";return call<PixPayment>(`/orders/${encodeURIComponent(reference)}/payments/pix`,{method:"POST",headers:{"X-CSRF-Token":csrf}})}
export async function paymentStatus(reference:string){return call<PaymentStatus>(`/orders/${encodeURIComponent(reference)}/payment`)}
export type CardPayment={AttemptID:string;Status:string;StatusDetail:string;Brand:string;LastFour:string;AuthenticationURL:string;AmountCents:number;Installments:number;RequiresAction:boolean};
export type CardTokenizedData={token:string;payment_method_id:string;installments:number;identification_type?:string;identification_number?:string};
export async function createCardPayment(reference:string,data:CardTokenizedData,key:string){const csrf=sessionStorage.getItem("loja3d_customer_csrf")||"";return call<CardPayment>(`/orders/${encodeURIComponent(reference)}/payments/card`,{method:"POST",headers:{"X-CSRF-Token":csrf,"Idempotency-Key":key},body:JSON.stringify(data)})}
