"use client";

import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { KeyRound, LockKeyhole, RefreshCw, ShieldCheck, UserPlus, Users, X } from "lucide-react";
import {
  adminRoles, createInternalUser, getInternalUsers, reauthenticateAdmin,
  resetInternalUserPassword, revokeInternalUserSessions, updateInternalUser,
  type AdminRole, type InternalUser,
} from "@/services/admin-api";
import { filterUsers, type UserFilters } from "./admin-users-state";

type SensitiveAction = (proof: string) => Promise<void>;
const roleLabels: Record<AdminRole, string> = { admin: "Administrador", quote_viewer: "Orçamentos", production_operator: "Operador de produção", production_manager: "Gerente de produção" };
const mfaLabels: Record<InternalUser["mfa_status"], string> = { enabled: "Habilitado", pending: "Pendente", not_configured: "Não configurado" };
const date = (value: string | null) => value ? new Date(value).toLocaleString("pt-BR") : "—";

function ReauthenticationDialog({ action, onClose, onSuccess }: { action: SensitiveAction | null; onClose: () => void; onSuccess: () => void }) {
  const ref = useRef<HTMLDialogElement>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => {
    const dialog = ref.current;
    if (action && !dialog?.open) dialog?.showModal();
    if (!action && dialog?.open) dialog.close();
  }, [action]);
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!action || busy) return;
    setBusy(true); setError("");
    const form = new FormData(event.currentTarget);
    try {
      const proof = await reauthenticateAdmin(String(form.get("password")), String(form.get("code") || ""), String(form.get("recovery_code") || ""));
      await action(proof.reauthentication_token);
      onSuccess(); onClose();
    } catch (cause) { setError(cause instanceof Error ? cause.message : "Reautenticação não concluída."); }
    finally { setBusy(false); }
  }
  return <dialog ref={ref} className="admin-dialog" onCancel={event => { event.preventDefault(); onClose(); }} aria-labelledby="reauth-title">
    <form onSubmit={submit}>
      <button type="button" className="admin-dialog-close" onClick={onClose} aria-label="Fechar"><X /></button>
      <span className="admin-kicker">Confirmação de identidade</span><h2 id="reauth-title">Reautenticação obrigatória</h2>
      <p>A evidência é mantida apenas em memória e consumida nesta operação.</p>
      <label>Senha atual<input name="password" type="password" autoComplete="current-password" required autoFocus /></label>
      <label>Código TOTP<input name="code" inputMode="numeric" autoComplete="one-time-code" pattern="[0-9]{6}" /></label>
      <label>Ou código de recuperação<input name="recovery_code" autoComplete="off" /></label>
      {error && <p className="admin-alert" role="alert">{error}</p>}
      <button className="admin-primary" disabled={busy}>{busy ? "Confirmando…" : "Confirmar e executar"}</button>
    </form>
  </dialog>;
}

export function AdminUsersManager({ initialUsers }: { initialUsers: InternalUser[] }) {
  const [users, setUsers] = useState(initialUsers);
  const [filters, setFilters] = useState<UserFilters>({ search: "", role: "", status: "", page: 1, limit: 8 });
  const [selected, setSelected] = useState<InternalUser | null>(null);
  const [creating, setCreating] = useState(false);
  const [pending, setPending] = useState<SensitiveAction | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const result = useMemo(() => filterUsers(users, filters), [users, filters]);
  async function reload() {
    setBusy(true); setError("");
    try {
      const next = await getInternalUsers(); setUsers(next);
      setSelected(current => current ? next.find(value => value.id === current.id) ?? null : null);
    } catch (cause) { setError(cause instanceof Error ? cause.message : "Não foi possível atualizar os usuários."); }
    finally { setBusy(false); }
  }
  function completed() { setMessage("Operação concluída com segurança."); setError(""); setNewPassword(""); void reload(); }
  function secure(action: SensitiveAction) { setPending(() => action); }
  function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); const form = event.currentTarget; const data = new FormData(form);
    secure(async proof => { await createInternalUser({ full_name: String(data.get("full_name")), email: String(data.get("email")), password: String(data.get("password")), role: String(data.get("role")) as AdminRole }, proof); form.reset(); setCreating(false); });
  }
  function change(user: InternalUser, role: AdminRole, status: "active" | "blocked") {
    if (!window.confirm(`Confirmar alteração da conta de ${user.full_name}?`)) return;
    secure(proof => updateInternalUser(user.id, { role, status }, proof).then(() => undefined));
  }
  function resetPassword(user: InternalUser) {
    if (newPassword.length < 12) { setError("A nova senha precisa ter ao menos 12 caracteres."); return; }
    if (!window.confirm("A redefinição revogará todas as sessões desse usuário. Continuar?")) return;
    secure(proof => resetInternalUserPassword(user.id, newPassword, proof));
  }
  function revoke(user: InternalUser) {
    if (!window.confirm(`Revogar todas as sessões ativas de ${user.full_name}?`)) return;
    secure(proof => revokeInternalUserSessions(user.id, proof));
  }
  return <>
    <section className="admin-users-toolbar"><div><span className="admin-kicker">Identidades internas</span><h2>{users.length} contas registradas</h2></div><div><button className="admin-secondary" onClick={reload} disabled={busy}><RefreshCw />Atualizar</button><button className="admin-primary" onClick={() => setCreating(value => !value)}><UserPlus />Novo usuário</button></div></section>
    {creating && <form className="admin-user-create" onSubmit={create}><h3>Criar conta interna</h3><p>A conta inicia ativa e configura o MFA obrigatório no primeiro acesso.</p><label>Nome completo<input name="full_name" required maxLength={150} /></label><label>E-mail<input name="email" type="email" required /></label><label>Role<select name="role" defaultValue="quote_viewer">{adminRoles.map(role => <option key={role} value={role}>{roleLabels[role]}</option>)}</select></label><label>Senha temporária<input name="password" type="password" minLength={12} maxLength={128} required autoComplete="new-password" /></label><button className="admin-primary">Reautenticar e criar</button></form>}
    <section className="admin-user-filters" aria-label="Filtros de usuários"><input aria-label="Buscar usuários" placeholder="Buscar nome ou e-mail" value={filters.search} onChange={event => setFilters(value => ({ ...value, search: event.target.value, page: 1 }))} /><select aria-label="Filtrar por role" value={filters.role} onChange={event => setFilters(value => ({ ...value, role: event.target.value as UserFilters["role"], page: 1 }))}><option value="">Todas as roles</option>{adminRoles.map(role => <option key={role} value={role}>{roleLabels[role]}</option>)}</select><select aria-label="Filtrar por status" value={filters.status} onChange={event => setFilters(value => ({ ...value, status: event.target.value as UserFilters["status"], page: 1 }))}><option value="">Todos os status</option><option value="active">Ativa</option><option value="blocked">Bloqueada</option></select></section>
    {message && <p className="admin-success" role="status">{message}</p>}{error && <p className="admin-alert" role="alert">{error}</p>}{busy && <p className="admin-inline-loading" role="status">Atualizando dados…</p>}
    {result.items.length ? <><div className="admin-table-wrap admin-users-table"><table className="admin-table"><thead><tr><th>Usuário</th><th>Role</th><th>Status</th><th>MFA</th><th>Sessões</th><th>Último acesso</th><th>Ações</th></tr></thead><tbody>{result.items.map(user => <tr key={user.id}><td><button className="admin-user-link" onClick={() => setSelected(user)}><strong>{user.full_name}</strong><small>{user.email}</small></button></td><td>{roleLabels[user.role]}</td><td><span className={`admin-badge account-${user.status}`}>{user.status === "active" ? "Ativa" : "Bloqueada"}</span></td><td><span className={`admin-badge mfa-${user.mfa_status}`}>{mfaLabels[user.mfa_status]}</span></td><td>{user.active_sessions}</td><td>{date(user.last_login_at)}</td><td><button className="admin-secondary compact" onClick={() => setSelected(user)}>Gerenciar</button></td></tr>)}</tbody></table></div><nav className="admin-pagination admin-page-buttons" aria-label="Paginação"><button disabled={result.page === 1} onClick={() => setFilters(value => ({ ...value, page: value.page - 1 }))}>Anterior</button><span>Página {result.page} de {result.pages} · {result.total} contas</span><button disabled={result.page === result.pages} onClick={() => setFilters(value => ({ ...value, page: value.page + 1 }))}>Próxima</button></nav></> : <div className="admin-empty"><Users /><h2>Nenhuma conta encontrada.</h2><p>Ajuste os filtros ou atualize os dados.</p></div>}
    {selected && <aside className="admin-user-drawer" aria-labelledby="user-detail-title"><button className="admin-dialog-close" onClick={() => setSelected(null)} aria-label="Fechar detalhes"><X /></button><span className="admin-kicker">Conta interna</span><h2 id="user-detail-title">{selected.full_name}</h2><p>{selected.email}</p><dl><div><dt>Status</dt><dd>{selected.status}</dd></div><div><dt>MFA</dt><dd>{mfaLabels[selected.mfa_status]}</dd></div><div><dt>Sessões ativas</dt><dd>{selected.active_sessions}</dd></div><div><dt>Criada em</dt><dd>{date(selected.created_at)}</dd></div><div><dt>Último acesso</dt><dd>{date(selected.last_login_at)}</dd></div></dl><label>Role<select value={selected.role} onChange={event => change(selected, event.target.value as AdminRole, selected.status)}>{adminRoles.map(role => <option key={role} value={role}>{roleLabels[role]}</option>)}</select></label><label>Nova senha temporária<input type="password" value={newPassword} onChange={event => setNewPassword(event.target.value)} minLength={12} maxLength={128} autoComplete="new-password" /></label><div className="admin-user-actions"><button className="admin-secondary" onClick={() => revoke(selected)}><ShieldCheck />Revogar sessões</button><button className="admin-secondary" onClick={() => resetPassword(selected)}><KeyRound />Redefinir senha</button><button className={selected.status === "active" ? "admin-danger" : "admin-primary"} onClick={() => change(selected, selected.role, selected.status === "active" ? "blocked" : "active")}><LockKeyhole />{selected.status === "active" ? "Bloquear conta" : "Desbloquear conta"}</button></div><p className="admin-user-note">Segredos TOTP e códigos de recuperação nunca são exibidos. Eventos detalhados de segurança não fazem parte do contrato atual.</p></aside>}
    <ReauthenticationDialog action={pending} onClose={() => setPending(null)} onSuccess={completed} />
  </>;
}
