import type{Metadata}from"next";import"./admin.css";import"./proposal.css";import"./catalog.css";import"./users.css";
export const metadata:Metadata={title:{default:"Administração",template:"%s | Admin Loja 3D"},robots:{index:false,follow:false}};
export default function AdminLayout({children}:{children:React.ReactNode}){return children}
