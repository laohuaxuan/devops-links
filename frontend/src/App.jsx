import { useCallback, useEffect, useMemo, useRef, useState } from "react";

const TOKEN_KEY = "devops_links_token";
const REMARK_MAX = 80;
const USERNAME_RULE = "用户名仅支持字母、数字、点、下划线、连字符，长度 2-64";
const PASSWORD_RULE = "密码需包含大小写字母、数字、特殊字符（!@#$%^&*），6-24 位";
const USERNAME_REGEX = /^[a-zA-Z0-9._-]{2,64}$/;

function isPasswordValid(password) {
  const v = String(password || "");
  if (v.length < 6 || v.length > 24) return false;
  return /[A-Z]/.test(v) && /[a-z]/.test(v) && /[0-9]/.test(v) && /[!@#$%^&*]/.test(v);
}

function validateUsernameInput(username) {
  const v = String(username || "").trim();
  if (!v) return "用户名不能为空";
  if (!USERNAME_REGEX.test(v)) return USERNAME_RULE;
  return "";
}

function validatePasswordInput(password) {
  const v = String(password || "").trim();
  if (!v) return "密码不能为空";
  if (!isPasswordValid(v)) return PASSWORD_RULE;
  return "";
}

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const RESERVED_USERNAMES = ["superadmin", "admin", "guest", "administrator", "root", "system"];

function validateEmailInput(email) {
  const v = String(email || "").trim();
  if (!v) return "邮箱不能为空";
  if (!EMAIL_REGEX.test(v)) return "邮箱格式无效";
  return "";
}

const ACTION_LABELS = {
  login: "超级管理员登录",
  login_ldap: "LDAP 登录",
  logout: "登出",
  create_user: "创建用户",
  update_user: "更新用户",
  delete_user: "删除用户",
  create_category: "创建分类",
  update_category: "更新分类",
  delete_category: "删除分类",
  create_link: "创建链接",
  update_link: "更新链接",
  delete_link: "删除链接",
  upload_icon: "上传图标",
  update_profile: "更新个人信息",
  reset_password: "重置密码",
  reset_user_password: "重置用户密码",
  register: "注册",
  reset_password_request: "找回密码",
  change_password: "修改密码",
};

async function api(path, options = {}, token = "") {
  const headers = { ...(options.headers || {}) };
  if (!(options.body instanceof FormData)) {
    headers["Content-Type"] = "application/json";
  }
  if (token) headers.Authorization = `Bearer ${token}`;
  const res = await fetch(path, { ...options, headers });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "请求失败");
  return data;
}

const TOAST_TTL_MS = 4000;
const TOAST_ERROR_PATTERN = /失败|无效|不能为空|错误|不能|已被|不一致|拒绝|禁用|过期|请填写|请输入/;

function resolveToastType(message, type) {
  if (type === "success" || type === "error") return type;
  return TOAST_ERROR_PATTERN.test(String(message || "")) ? "error" : "success";
}

function ToastBanner({ toast, onClose }) {
  if (!toast?.text) return null;
  return (
    <div className="toast-banner-wrap" aria-live="polite">
      <div className={`toast-banner ${toast.type}`} role="status">
        <span className="toast-banner-text">{toast.text}</span>
        <button type="button" className="toast-banner-close" aria-label="关闭提示" onClick={onClose}>
          ×
        </button>
      </div>
    </div>
  );
}

function useToast() {
  const [toast, setToast] = useState(null);
  const timerRef = useRef(null);

  const dismissToast = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    setToast(null);
  }, []);

  const showToast = useCallback((message, type) => {
    const text = String(message || "").trim();
    if (!text) return;
    if (timerRef.current) clearTimeout(timerRef.current);
    setToast({ text, type: resolveToastType(text, type) });
    timerRef.current = setTimeout(() => {
      timerRef.current = null;
      setToast(null);
    }, TOAST_TTL_MS);
  }, []);

  useEffect(() => () => {
    if (timerRef.current) clearTimeout(timerRef.current);
  }, []);

  return { toast, showToast, dismissToast };
}

function truncateRemark(text, max = REMARK_MAX) {
  const v = String(text || "").trim();
  if (!v) return { display: "", full: "" };
  if (v.length <= max) return { display: v, full: v };
  return { display: `${v.slice(0, max)}…`, full: v };
}

function formatTime(value) {
  if (!value) return "-";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  return d.toLocaleString();
}

function defaultLinkForm(categoryId = "") {
  return {
    category_id: categoryId ? String(categoryId) : "",
    name: "",
    url: "",
    icon_path: "",
    icon_url: "",
    maintainer: "",
    remark: "",
    sort_order: "0",
  };
}

function findCategoryById(categories, id) {
  if (!id) return null;
  for (const cat of categories) {
    if (cat.id === id) return cat;
    for (const child of cat.children || []) {
      if (child.id === id) return child;
    }
  }
  return null;
}

function getDisplayLinks(category) {
  if (!category) return [];
  if (!category.parent_id) {
    const childLinks = (category.children || []).flatMap((child) => child.links || []);
    return [...(category.links || []), ...childLinks];
  }
  return category.links || [];
}

function linkTargetCategories(categories) {
  const out = [];
  for (const root of categories) {
    if (root.can_manage) {
      out.push({ id: root.id, label: root.name });
    }
    for (const child of root.children || []) {
      if (child.can_manage) {
        out.push({ id: child.id, label: `${root.name} / ${child.name}` });
      }
    }
  }
  return out;
}

function findParentCategoryId(categories, categoryId) {
  for (const cat of categories) {
    if (cat.id === categoryId) {
      return null;
    }
    for (const child of cat.children || []) {
      if (child.id === categoryId) {
        return cat.id;
      }
    }
  }
  return null;
}

function categoryTypeLabel(category) {
  if (!category) return "";
  return category.parent_id ? "子分类" : "大分类";
}

function defaultUserForm() {
  return {
    name: "",
    display_name: "",
    email: "",
    password: "",
    confirm_password: "",
    role: "guest",
  };
}

function LinkCard({ link, onEdit, onDelete }) {
  const remark = truncateRemark(link.remark);
  return (
    <article className="link-card">
      <a href={link.url} target="_blank" rel="noreferrer" className="link-card-main">
        <div className="link-icon-wrap">
          {link.icon_url ? (
            <img src={link.icon_url} alt="" className="link-icon" />
          ) : (
            <span className="link-icon-fallback">{link.name?.slice(0, 1) || "?"}</span>
          )}
        </div>
        <div className="link-body">
          <h3>{link.name}</h3>
          <p className="link-url">{link.url}</p>
          {link.maintainer ? <p className="link-meta">维护人：{link.maintainer}</p> : null}
          {remark.display ? (
            <p className="link-remark" title={remark.full}>
              {remark.display}
            </p>
          ) : null}
        </div>
      </a>
      {link.can_manage ? (
        <div className="link-actions">
          <button type="button" className="link-btn" onClick={() => onEdit(link)}>
            编辑
          </button>
          <button type="button" className="link-btn danger" onClick={() => onDelete(link)}>
            删除
          </button>
        </div>
      ) : null}
    </article>
  );
}

function UsersPanel({ token, showToast }) {
  const [users, setUsers] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [keyword, setKeyword] = useState("");
  const [search, setSearch] = useState("");
  const [modal, setModal] = useState(null);
  const [form, setForm] = useState(defaultUserForm());
  const [resetPasswordModal, setResetPasswordModal] = useState(null);
  const pageSize = 20;

  const loadUsers = useCallback(async () => {
    const params = new URLSearchParams({ page: String(page), size: String(pageSize) });
    if (search) params.set("keyword", search);
    const data = await api(`/api/users?${params}`, {}, token);
    setUsers(data.users || []);
    setTotal(data.total || 0);
  }, [token, page, search]);

  useEffect(() => {
    loadUsers().catch((err) => showToast(err.message));
  }, [loadUsers, showToast]);

  function openCreate() {
    setForm(defaultUserForm());
    setModal({ mode: "create" });
  }

  function openEdit(user) {
    setForm({
      id: user.id,
      name: user.name,
      display_name: user.display_name || user.name,
      email: user.email || "",
      role: user.role,
      status: user.status || "active",
      auth_source: user.auth_source,
      password: "",
      confirm_password: "",
    });
    setModal({ mode: "edit", user });
  }

  async function saveUser(e) {
    e.preventDefault();
    try {
      if (modal.mode === "create") {
        const nameMsg = validateUsernameInput(form.name);
        if (nameMsg) {
          showToast(nameMsg);
          return;
        }
        const pwdMsg = validatePasswordInput(form.password);
        if (pwdMsg) {
          showToast(pwdMsg);
          return;
        }
        if (form.password !== form.confirm_password) {
          showToast("两次输入的密码不一致");
          return;
        }
        if (form.email.trim()) {
          const emailMsg = validateEmailInput(form.email);
          if (emailMsg) {
            showToast(emailMsg);
            return;
          }
        }
        await api("/api/users", { method: "POST", body: JSON.stringify(form) }, token);
        showToast("用户已创建");
      } else {
        const payload = {
          display_name: form.display_name,
          role: form.role,
          status: form.status,
        };
        await api(`/api/users/${form.id}`, { method: "PUT", body: JSON.stringify(payload) }, token);
        showToast("用户已更新");
      }
      setModal(null);
      await loadUsers();
    } catch (err) {
      showToast(err.message);
    }
  }

  async function resetUserPassword(user) {
    if (user.auth_source === "ldap") {
      showToast("LDAP 用户请在域内修改密码");
      return;
    }
    if (!window.confirm(`确认重置用户「${user.display_name || user.name}」的密码？该用户将被强制重新登录。`)) return;
    try {
      const data = await api(`/api/users/${user.id}/reset-password`, { method: "POST" }, token);
      setResetPasswordModal({ user, password: data.password || "" });
      showToast("密码已重置");
      await loadUsers();
    } catch (err) {
      showToast(err.message);
    }
  }

  async function copyResetPassword() {
    if (!resetPasswordModal?.password) return;
    try {
      await navigator.clipboard.writeText(resetPasswordModal.password);
      showToast("已复制到剪贴板");
    } catch {
      showToast("复制失败，请手动复制");
    }
  }

  async function deleteUser(user) {
    if (!window.confirm(`确认删除用户「${user.display_name || user.name}」？`)) return;
    try {
      await api(`/api/users/${user.id}`, { method: "DELETE" }, token);
      showToast("用户已删除");
      await loadUsers();
    } catch (err) {
      showToast(err.message);
    }
  }

  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  return (
    <div className="admin-panel">
      <div className="panel-toolbar">
        <h2>用户管理</h2>
        <div className="toolbar-actions">
          <input
            className="search-input"
            placeholder="搜索用户名/显示名/邮箱"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                setPage(1);
                setSearch(keyword.trim());
              }
            }}
          />
          <button type="button" onClick={() => { setPage(1); setSearch(keyword.trim()); }}>
            搜索
          </button>
          <button type="button" onClick={openCreate}>+ 新建用户</button>
        </div>
      </div>
      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>用户名</th>
              <th>显示名</th>
              <th>邮箱</th>
              <th>角色</th>
              <th>来源</th>
              <th>状态</th>
              <th>创建时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {users.map((u) => (
              <tr key={u.id}>
                <td>{u.name}</td>
                <td>{u.display_name || u.name}</td>
                <td>{u.email || "-"}</td>
                <td><span className={`badge ${roleBadgeClass(u.role)}`}>{roleLabel(u.role)}</span></td>
                <td>{u.auth_source === "ldap" ? "LDAP" : "本地"}</td>
                <td><span className={`badge status-${u.status}`}>{u.status === "disabled" ? "禁用" : "正常"}</span></td>
                <td>{formatTime(u.created_at)}</td>
                <td className="table-actions">
                  <button type="button" className="link-btn" onClick={() => openEdit(u)}>编辑</button>
                  {u.auth_source !== "ldap" ? (
                    <button type="button" className="link-btn" onClick={() => resetUserPassword(u)}>重置密码</button>
                  ) : null}
                  <button type="button" className="link-btn danger" onClick={() => deleteUser(u)}>删除</button>
                </td>
              </tr>
            ))}
            {users.length === 0 ? (
              <tr><td colSpan={8} className="empty-cell">暂无用户</td></tr>
            ) : null}
          </tbody>
        </table>
      </div>
      <div className="pager">
        <button type="button" className="btn-secondary" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>上一页</button>
        <span>{page} / {totalPages}（共 {total} 条）</span>
        <button type="button" className="btn-secondary" disabled={page >= totalPages} onClick={() => setPage((p) => p + 1)}>下一页</button>
      </div>

      {modal ? (
        <div className="modal-overlay" onClick={() => setModal(null)}>
          <form className="modal modal-wide" onClick={(e) => e.stopPropagation()} onSubmit={saveUser}>
            <h3>{modal.mode === "create" ? "新建用户" : "编辑用户"}</h3>
            {modal.mode === "create" ? (
              <label>
                用户名 *
                <input required value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="字母、数字、._-" />
              </label>
            ) : (
              <label>
                用户名
                <input value={form.name} disabled />
              </label>
            )}
            <label>
              显示名 *
              <input required value={form.display_name} onChange={(e) => setForm({ ...form, display_name: e.target.value })} />
            </label>
            {modal.mode === "create" ? (
              <label>
                邮箱
                <input type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} placeholder="可选，用于找回密码" />
              </label>
            ) : (
              <label>
                邮箱
                <input value={form.email || "-"} disabled />
              </label>
            )}
            <label>
              角色 *
              <select required value={form.role} onChange={(e) => setForm({ ...form, role: e.target.value })}>
                <option value="guest">游客</option>
                <option value="admin">管理员</option>
                <option value="superadmin">超级管理员</option>
              </select>
            </label>
            {modal.mode === "create" ? (
              <>
                <p className="hint-text">{PASSWORD_RULE}</p>
                <label>
                  密码 *
                  <input
                    type="password"
                    required
                    value={form.password}
                    onChange={(e) => setForm({ ...form, password: e.target.value })}
                  />
                </label>
                <label>
                  确认密码 *
                  <input
                    type="password"
                    required
                    value={form.confirm_password}
                    onChange={(e) => setForm({ ...form, confirm_password: e.target.value })}
                  />
                </label>
              </>
            ) : null}
            {modal.mode === "edit" ? (
              <label>
                状态 *
                <select required value={form.status} onChange={(e) => setForm({ ...form, status: e.target.value })}>
                  <option value="active">正常</option>
                  <option value="disabled">禁用</option>
                </select>
              </label>
            ) : null}
            {modal.mode === "edit" && form.auth_source === "ldap" ? (
              <p className="hint-text">LDAP 用户请在域内修改密码，可使用「重置密码」生成新密码</p>
            ) : null}
            <div className="modal-actions">
              <button type="button" className="btn-secondary" onClick={() => setModal(null)}>取消</button>
              <button type="submit">保存</button>
            </div>
          </form>
        </div>
      ) : null}

      {resetPasswordModal ? (
        <div className="modal-overlay modal-overlay-top" onClick={() => setResetPasswordModal(null)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h3>已重置用户密码</h3>
            <p className="hint-text">
              用户「{resetPasswordModal.user.display_name || resetPasswordModal.user.name}」的新密码如下。该用户现有登录已失效，需重新登录。
            </p>
            <div className="generated-password">{resetPasswordModal.password}</div>
            <div className="modal-actions">
              <button type="button" className="btn-secondary" onClick={() => setResetPasswordModal(null)}>关闭</button>
              <button type="button" onClick={copyResetPassword}>复制密码</button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}

function roleLabel(role) {
  if (role === "superadmin") return "超级管理员";
  if (role === "admin") return "管理员";
  return "游客";
}

function roleBadgeClass(role) {
  if (role === "superadmin") return "role-superadmin";
  if (role === "admin") return "role-admin";
  return "role-guest";
}

function UserMenu({ user, onProfile, onLogout }) {
  const [open, setOpen] = useState(false);
  const wrapRef = useRef(null);
  const closeTimerRef = useRef(null);
  const leaveTimerRef = useRef(null);
  const displayName = user?.display_name || user?.name || "用户";
  const avatarChar = displayName.trim().slice(0, 1) || "?";

  const clearCloseTimer = useCallback(() => {
    if (closeTimerRef.current) {
      clearTimeout(closeTimerRef.current);
      closeTimerRef.current = null;
    }
  }, []);

  const clearLeaveTimer = useCallback(() => {
    if (leaveTimerRef.current) {
      clearTimeout(leaveTimerRef.current);
      leaveTimerRef.current = null;
    }
  }, []);

  const scheduleAutoClose = useCallback(() => {
    clearCloseTimer();
    closeTimerRef.current = setTimeout(() => setOpen(false), 3000);
  }, [clearCloseTimer]);

  useEffect(() => () => {
    clearCloseTimer();
    clearLeaveTimer();
  }, [clearCloseTimer, clearLeaveTimer]);

  function handleMouseEnter() {
    clearLeaveTimer();
    setOpen(true);
    scheduleAutoClose();
  }

  function handleMouseLeave() {
    clearLeaveTimer();
    leaveTimerRef.current = setTimeout(() => {
      clearCloseTimer();
      setOpen(false);
    }, 120);
  }

  function handleMenuAction(action) {
    clearCloseTimer();
    clearLeaveTimer();
    setOpen(false);
    action();
  }

  return (
    <div
      className="user-menu-wrap"
      ref={wrapRef}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
    >
      <div className="user-menu-trigger" aria-haspopup="true" aria-expanded={open}>
        <span className="user-avatar">{avatarChar}</span>
        <span className="user-menu-info">
          <span className="user-menu-name">{displayName}</span>
          <span className="user-menu-role">{roleLabel(user?.role)}</span>
        </span>
      </div>
      {open ? (
        <div className="user-menu-dropdown">
          <button
            type="button"
            className="user-menu-item"
            onClick={() => handleMenuAction(onProfile)}
          >
            个人设置
          </button>
          <button
            type="button"
            className="user-menu-item danger"
            onClick={() => handleMenuAction(onLogout)}
          >
            退出登录
          </button>
        </div>
      ) : null}
    </div>
  );
}

function EyeToggleIcon({ open }) {
  if (open) {
    return (
      <svg className="eye-icon" viewBox="0 0 24 24" aria-hidden="true">
        <path
          fill="currentColor"
          d="M12 5c-5 0-9.27 3.11-11 7 1.73 3.89 6 7 11 7s9.27-3.11 11-7c-1.73-3.89-6-7-11-7zm0 12a5 5 0 1 1 0-10 5 5 0 0 1 0 10zm0-8a3 3 0 1 0 .001 6.001A3 3 0 0 0 12 9z"
        />
      </svg>
    );
  }
  return (
    <svg className="eye-icon" viewBox="0 0 24 24" aria-hidden="true">
      <path
        fill="currentColor"
        d="M2.1 3.51 3.51 2.1l18.38 18.39-1.41 1.41-3.05-3.05A12.4 12.4 0 0 1 12 19c-5 0-9.27-3.11-11-7a13.3 13.3 0 0 1 4.2-5.17L2.1 3.51zM12 7a5 5 0 0 1 4.9 3.96l-1.55-1.55A3 3 0 0 0 12 9c-.38 0-.74.08-1.07.21L9.3 7.58A4.9 4.9 0 0 1 12 7zm-8.86 5c.98 1.98 3.05 3.84 5.94 4.7l-1.7-1.7A5 5 0 0 1 7.1 9.96L4.4 7.26A11.5 11.5 0 0 0 3.14 12zM14.9 12.8l-2.7-2.7.1-.1a3 3 0 0 1 2.6 2.8z"
      />
    </svg>
  );
}

function LoginRegister({ onLogin, showToast }) {
  const [loading, setLoading] = useState(false);
  const [loginError, setLoginError] = useState("");
  const [loginForm, setLoginForm] = useState({ username: "", password: "" });
  const [loginFieldErrors, setLoginFieldErrors] = useState({ username: "", password: "" });
  const [loginPasswordVisible, setLoginPasswordVisible] = useState(false);

  useEffect(() => {
    const path = window.location.pathname || "";
    if (path.startsWith("/register") || path.startsWith("/reset-password")) {
      window.history.replaceState({}, "", "/login");
    }
  }, []);

  async function handleLogin(e) {
    e.preventDefault();
    setLoginError("");
    const usernameError = validateUsernameInput(loginForm.username);
    const passwordError = loginForm.password ? "" : "密码不能为空";
    setLoginFieldErrors({ username: usernameError, password: passwordError });
    if (usernameError || passwordError) return;
    setLoading(true);
    try {
      const data = await api("/api/login", {
        method: "POST",
        body: JSON.stringify({
          username: loginForm.username.trim(),
          password: loginForm.password,
        }),
      });
      window.history.replaceState({}, "", "/");
      onLogin(data);
    } catch (err) {
      setLoginError(err.message);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="login-page">
      <div className="login-card">
        <h1>DevOps 链接导航</h1>
        <h2 className="login-heading">使用AD域账号进行验证</h2>
        <form className="login-form" onSubmit={handleLogin}>
          <label>
            用户名
            <input
              value={loginForm.username}
              onChange={(e) => {
                const username = e.target.value;
                setLoginForm({ ...loginForm, username });
                setLoginFieldErrors((prev) => ({ ...prev, username: username ? validateUsernameInput(username) : "" }));
              }}
              placeholder="请输入用户名"
              autoComplete="username"
              required
            />
            {loginFieldErrors.username ? <span className="field-error">{loginFieldErrors.username}</span> : null}
          </label>
          <label>
            密码
            <div className="password-input-row auth-password-row">
              <input
                type={loginPasswordVisible ? "text" : "password"}
                value={loginForm.password}
                onChange={(e) => {
                  const password = e.target.value;
                  setLoginForm({ ...loginForm, password });
                  setLoginFieldErrors((prev) => ({ ...prev, password: password ? "" : "密码不能为空" }));
                }}
                placeholder="请输入密码"
                autoComplete="current-password"
                required
              />
              <button
                type="button"
                className="password-toggle-btn"
                title={loginPasswordVisible ? "隐藏密码" : "显示密码"}
                aria-label={loginPasswordVisible ? "隐藏密码" : "显示密码"}
                onClick={() => setLoginPasswordVisible((v) => !v)}
              >
                <EyeToggleIcon open={loginPasswordVisible} />
              </button>
            </div>
            {loginFieldErrors.password ? <span className="field-error">{loginFieldErrors.password}</span> : null}
          </label>
          {loginError ? <p className="error-text">{loginError}</p> : null}
          <button type="submit" className="login-submit-btn" disabled={loading}>
            {loading ? "登录中..." : "登录"}
          </button>
        </form>
      </div>
    </div>
  );
}

function ProfileModal({ open, onClose, token, user, onUpdated, onRequireRelogin, showToast }) {
  const isLDAP = user?.auth_source === "ldap";
  const [form, setForm] = useState({
    display_name: user?.display_name || user?.name || "",
    email: user?.email || "",
    new_password: "",
    confirm_password: "",
  });
  const [saving, setSaving] = useState(false);
  const [displayNameMsg, setDisplayNameMsg] = useState("");
  const [displayNameOk, setDisplayNameOk] = useState(false);
  const [emailMsg, setEmailMsg] = useState("");
  const [emailOk, setEmailOk] = useState(false);

  const newPasswordError = form.new_password ? validatePasswordInput(form.new_password) : "";
  const confirmPasswordError =
    form.confirm_password && form.new_password !== form.confirm_password
      ? "两次输入的新密码不一致"
      : "";

  useEffect(() => {
    if (!open) return;
    setForm({
      display_name: user?.display_name || user?.name || "",
      email: user?.email || "",
      new_password: "",
      confirm_password: "",
    });
  }, [open, user]);

  useEffect(() => {
    if (!open || !isLDAP) return;
    onUpdated().catch(() => {});
  }, [open, isLDAP, onUpdated]);

  useEffect(() => {
    if (!open) return;
    setDisplayNameMsg("");
    setDisplayNameOk(false);
    setEmailMsg("");
    setEmailOk(false);
  }, [open]);

  function clearDisplayNameFeedback() {
    setDisplayNameMsg("");
    setDisplayNameOk(false);
  }

  function clearEmailFeedback() {
    setEmailMsg("");
    setEmailOk(false);
  }

  async function saveProfile(e) {
    e.preventDefault();
    clearDisplayNameFeedback();
    clearEmailFeedback();

    const displayName = String(form.display_name || "").trim();
    const email = String(form.email || "").trim();
    if (!displayName) {
      setDisplayNameMsg("显示名不能为空");
      setDisplayNameOk(false);
      return;
    }
    if (!isLDAP) {
      const emailMsgText = validateEmailInput(email);
      if (emailMsgText) {
        setEmailMsg(emailMsgText);
        setEmailOk(false);
        return;
      }
      if (form.new_password || form.confirm_password) {
        const pwdMsg = validatePasswordInput(form.new_password);
        if (pwdMsg) {
          showToast(pwdMsg);
          return;
        }
        if (form.new_password !== form.confirm_password) {
          showToast("两次输入的新密码不一致");
          return;
        }
      }
    }
    setSaving(true);
    try {
      const data = await api("/api/profile", {
        method: "PUT",
        body: JSON.stringify({
          display_name: displayName,
          email,
          new_password: form.new_password,
          confirm_password: form.confirm_password,
        }),
      }, token);
      if (data.password_changed) {
        showToast("密码已修改，请重新登录");
        onClose();
        onRequireRelogin();
        return;
      }
      setDisplayNameMsg("显示名已保存");
      setDisplayNameOk(true);
      setEmailMsg("邮箱已保存");
      setEmailOk(true);
      const profilePatch = {
        display_name: data.display_name ?? displayName,
        email: data.email ?? email,
      };
      setForm((f) => ({
        ...f,
        ...profilePatch,
        new_password: "",
        confirm_password: "",
      }));
      await onUpdated(profilePatch);
    } catch (err) {
      const msg = err.message || "保存失败";
      if (msg.includes("显示名")) {
        setDisplayNameMsg(msg);
        setDisplayNameOk(false);
      } else if (msg.includes("邮箱") || msg.includes("LDAP")) {
        setEmailMsg(msg);
        setEmailOk(false);
      } else {
        showToast(msg);
      }
    } finally {
      setSaving(false);
    }
  }

  if (!open) return null;

  return (
    <div className="modal-overlay" onClick={() => !saving && onClose()}>
      <form
        className="modal modal-wide profile-modal"
        onClick={(e) => e.stopPropagation()}
        onSubmit={saveProfile}
      >
        <h3>个人设置</h3>
        <label>
          用户名
          <input value={user?.name || ""} disabled />
        </label>
        <label>
          邮箱 *
          <input
            required
            type="email"
            value={form.email}
            disabled={isLDAP}
            onChange={(e) => {
              setForm({ ...form, email: e.target.value });
              clearEmailFeedback();
            }}
          />
          {emailMsg ? <span className={emailOk ? "field-hint" : "field-error"}>{emailMsg}</span> : null}
        </label>
        <label>
          显示名 *
          <input
            required
            value={form.display_name}
            onChange={(e) => {
              setForm({ ...form, display_name: e.target.value });
              clearDisplayNameFeedback();
            }}
          />
          {displayNameMsg ? <span className={displayNameOk ? "field-hint" : "field-error"}>{displayNameMsg}</span> : null}
        </label>
        <label>
          登录方式
          <input value={isLDAP ? "LDAP" : "本地"} disabled />
        </label>
        {!isLDAP ? (
          <>
            <div className="profile-section">
              <h4>修改密码</h4>
              <p className="hint-text">{PASSWORD_RULE}。留空表示不修改。</p>
            </div>
            <label>
              新密码
              <input
                type="password"
                value={form.new_password}
                onChange={(e) => setForm({ ...form, new_password: e.target.value })}
                autoComplete="new-password"
              />
              {newPasswordError ? <span className="field-error">{newPasswordError}</span> : null}
            </label>
            <label>
              确认新密码
              <input
                type="password"
                value={form.confirm_password}
                onChange={(e) => setForm({ ...form, confirm_password: e.target.value })}
                autoComplete="new-password"
              />
              {confirmPasswordError ? <span className="field-error">{confirmPasswordError}</span> : null}
            </label>
          </>
        ) : (
          <p className="hint-text">LDAP 用户请在域内修改密码，邮箱由 AD 同步。</p>
        )}
        <div className="modal-actions">
          <button type="button" className="btn-secondary" disabled={saving} onClick={onClose}>
            取消
          </button>
          <button type="submit" disabled={saving}>{saving ? "保存中..." : "保存"}</button>
        </div>
      </form>
    </div>
  );
}

function AuditPanel({ token, showToast }) {
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [keyword, setKeyword] = useState("");
  const [search, setSearch] = useState("");
  const [startAt, setStartAt] = useState("");
  const [endAt, setEndAt] = useState("");
  const pageSize = 20;

  const loadLogs = useCallback(async () => {
    const params = new URLSearchParams({ page: String(page), size: String(pageSize) });
    if (search) params.set("keyword", search);
    if (startAt) params.set("start_at", startAt);
    if (endAt) params.set("end_at", endAt);
    const data = await api(`/api/audit-logs?${params}`, {}, token);
    setItems(data.items || []);
    setTotal(data.total || 0);
  }, [token, page, search, startAt, endAt]);

  useEffect(() => {
    loadLogs().catch((err) => showToast(err.message));
  }, [loadLogs, showToast]);

  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  return (
    <div className="admin-panel">
      <div className="panel-toolbar">
        <h2>审计日志</h2>
        <div className="toolbar-actions audit-filters">
          <input className="search-input" placeholder="关键词" value={keyword} onChange={(e) => setKeyword(e.target.value)} />
          <input type="datetime-local" value={startAt} onChange={(e) => setStartAt(e.target.value)} />
          <input type="datetime-local" value={endAt} onChange={(e) => setEndAt(e.target.value)} />
          <button type="button" onClick={() => { setPage(1); setSearch(keyword.trim()); }}>查询</button>
        </div>
      </div>
      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>时间</th>
              <th>用户</th>
              <th>操作</th>
              <th>结果</th>
              <th>IP</th>
              <th>详情</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.id}>
                <td>{formatTime(item.created_at)}</td>
                <td>{item.display_name || item.username || "-"}</td>
                <td>{ACTION_LABELS[item.action] || item.action}</td>
                <td><span className={`badge result-${item.result}`}>{item.result === "success" ? "成功" : item.result}</span></td>
                <td>{item.ip || "-"}</td>
                <td className="detail-cell" title={item.detail}>{item.detail || "-"}</td>
              </tr>
            ))}
            {items.length === 0 ? (
              <tr><td colSpan={6} className="empty-cell">暂无日志</td></tr>
            ) : null}
          </tbody>
        </table>
      </div>
      <div className="pager">
        <button type="button" className="btn-secondary" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>上一页</button>
        <span>{page} / {totalPages}（共 {total} 条）</span>
        <button type="button" className="btn-secondary" disabled={page >= totalPages} onClick={() => setPage((p) => p + 1)}>下一页</button>
      </div>
    </div>
  );
}

export default function App() {
  const [token, setToken] = useState(() => localStorage.getItem(TOKEN_KEY) || "");
  const [user, setUser] = useState(null);
  const [categories, setCategories] = useState([]);
  const [selectedCategoryId, setSelectedCategoryId] = useState(null);
  const [expandedCategoryIds, setExpandedCategoryIds] = useState(() => new Set());
  const { toast, showToast, dismissToast } = useToast();
  const [categoryModal, setCategoryModal] = useState(null);
  const [linkModal, setLinkModal] = useState(null);
  const [linkForm, setLinkForm] = useState(defaultLinkForm());
  const [uploading, setUploading] = useState(false);
  const [activeTab, setActiveTab] = useState("links");
  const [profileModalOpen, setProfileModalOpen] = useState(false);

  const isAdmin = user?.is_admin === true;
  const isSuperAdmin = user?.is_super_admin === true;

  const selectedCategory = useMemo(
    () => findCategoryById(categories, selectedCategoryId),
    [categories, selectedCategoryId]
  );

  const displayLinks = useMemo(
    () => getDisplayLinks(selectedCategory),
    [selectedCategory]
  );

  const manageableLinkCategories = useMemo(
    () => linkTargetCategories(categories),
    [categories]
  );

  useEffect(() => {
    if (!selectedCategoryId || !categories.length) return;
    const parentId = findParentCategoryId(categories, selectedCategoryId);
    if (!parentId) return;
    setExpandedCategoryIds((prev) => {
      if (prev.has(parentId)) return prev;
      const next = new Set(prev);
      next.add(parentId);
      return next;
    });
  }, [selectedCategoryId, categories]);

  function toggleCategoryExpand(catId) {
    setExpandedCategoryIds((prev) => {
      const next = new Set(prev);
      if (next.has(catId)) {
        next.delete(catId);
      } else {
        next.add(catId);
      }
      return next;
    });
  }

  function selectCategory(categoryId, parentId = null) {
    setSelectedCategoryId(categoryId);
    if (parentId) {
      setExpandedCategoryIds((prev) => {
        if (prev.has(parentId)) return prev;
        const next = new Set(prev);
        next.add(parentId);
        return next;
      });
    }
  }

  const loadCategories = useCallback(async (tok = token) => {
    const data = await api("/api/categories", {}, tok);
    const nextCategories = data.categories || [];
    setCategories(nextCategories);
    if (nextCategories.length) {
      setSelectedCategoryId((prev) => {
        if (prev && findCategoryById(nextCategories, prev)) return prev;
        return nextCategories[0].id;
      });
    } else {
      setSelectedCategoryId(null);
    }
  }, [token]);

  const refreshUser = useCallback(async (profilePatch, tok = token) => {
    if (profilePatch && typeof profilePatch === "object") {
      setUser((current) => (current ? { ...current, ...profilePatch } : current));
    }
    const data = await api("/api/me", {}, tok);
    setUser(data);
    return data;
  }, [token]);

  useEffect(() => {
    if (!user) return;
    if ((activeTab === "users" || activeTab === "audit") && !user.is_super_admin) {
      setActiveTab("links");
    }
  }, [user, activeTab]);

  useEffect(() => {
    if (!token) {
      setUser(null);
      return;
    }
    api("/api/me", {}, token)
      .then((data) => {
        setUser(data);
        return loadCategories();
      })
      .catch(() => {
        localStorage.removeItem(TOKEN_KEY);
        setToken("");
        setUser(null);
      });
  }, [token, loadCategories]);

  function handleAuthLogin(data) {
    localStorage.setItem(TOKEN_KEY, data.token);
    setToken(data.token);
    setUser(data.user);
    loadCategories(data.token).catch(() => {});
  }

  async function logout() {
    try {
      if (token) await api("/api/logout", { method: "POST" }, token);
    } catch {
      // ignore
    }
    localStorage.removeItem(TOKEN_KEY);
    setToken("");
    setUser(null);
    setCategories([]);
    setActiveTab("links");
    setProfileModalOpen(false);
  }

  async function saveCategory(e) {
    e.preventDefault();
    const name = String(categoryModal?.name || "").trim();
    if (!name) {
      showToast("分类名称不能为空");
      return;
    }
    const payload = {
      name,
      sort_order: Number(categoryModal?.sort_order || 0),
      parent_id: Number(categoryModal?.parent_id || 0),
    };
    try {
      if (categoryModal?.id) {
        await api(`/api/categories/${categoryModal.id}`, { method: "PUT", body: JSON.stringify(payload) }, token);
      } else {
        await api("/api/categories", { method: "POST", body: JSON.stringify(payload) }, token);
      }
      setCategoryModal(null);
      showToast("分类已保存");
      await loadCategories();
    } catch (err) {
      showToast(err.message);
    }
  }

  function openCreateRootCategory() {
    if (!isAdmin) return;
    if (!isSuperAdmin) {
      showToast("请联系超级管理员创建大分类", "error");
      return;
    }
    setCategoryModal({ name: "", sort_order: 0, parent_id: 0 });
  }

  function openCreateSubCategory(parentId) {
    if (!isAdmin) return;
    setExpandedCategoryIds((prev) => {
      const next = new Set(prev);
      next.add(parentId);
      return next;
    });
    setCategoryModal({ name: "", sort_order: 0, parent_id: parentId });
  }

  async function deleteCategory(cat) {
    if (!window.confirm(`确认删除分类「${cat.name}」及其下所有链接？`)) return;
    try {
      await api(`/api/categories/${cat.id}`, { method: "DELETE" }, token);
      showToast("分类已删除");
      await loadCategories();
    } catch (err) {
      showToast(err.message);
    }
  }

  function openLinkModal(link = null) {
    if (link) {
      setLinkForm({
        id: link.id,
        category_id: String(link.category_id),
        name: link.name || "",
        url: link.url || "",
        icon_path: link.icon_path || "",
        icon_url: link.icon_url || "",
        maintainer: link.maintainer || "",
        remark: link.remark || "",
        sort_order: String(link.sort_order ?? 0),
      });
    } else {
      setLinkForm(defaultLinkForm(selectedCategory?.id || ""));
    }
    setLinkModal(true);
  }

  async function uploadIcon(file) {
    if (!file) return;
    setUploading(true);
    try {
      const fd = new FormData();
      fd.append("file", file);
      const data = await api("/api/upload/icon", { method: "POST", body: fd }, token);
      setLinkForm((f) => ({ ...f, icon_path: data.icon_path, icon_url: data.icon_url }));
      showToast("图标上传成功");
    } catch (err) {
      showToast(err.message);
    } finally {
      setUploading(false);
    }
  }

  function clearLinkIcon() {
    setLinkForm((f) => ({ ...f, icon_path: "", icon_url: "" }));
  }

  async function saveLink(e) {
    e.preventDefault();
    const payload = {
      category_id: Number(linkForm.category_id),
      name: String(linkForm.name || "").trim(),
      url: String(linkForm.url || "").trim(),
      icon_path: String(linkForm.icon_path || "").trim(),
      maintainer: String(linkForm.maintainer || "").trim(),
      remark: String(linkForm.remark || "").trim(),
      sort_order: Number(linkForm.sort_order || 0),
    };
    try {
      if (linkForm.id) {
        await api(`/api/links/${linkForm.id}`, { method: "PUT", body: JSON.stringify(payload) }, token);
      } else {
        await api("/api/links", { method: "POST", body: JSON.stringify(payload) }, token);
      }
      setLinkModal(false);
      showToast("链接已保存");
      await loadCategories();
    } catch (err) {
      showToast(err.message);
    }
  }

  async function deleteLink(link) {
    if (!window.confirm(`确认删除链接「${link.name}」？`)) return;
    try {
      await api(`/api/links/${link.id}`, { method: "DELETE" }, token);
      showToast("链接已删除");
      await loadCategories();
    } catch (err) {
      showToast(err.message);
    }
  }

  if (!token || !user) {
    return (
      <>
        <ToastBanner toast={toast} onClose={dismissToast} />
        <LoginRegister onLogin={handleAuthLogin} showToast={showToast} />
      </>
    );
  }

  return (
    <div className="app-shell">
      <header className="topbar">
        <div>
          <h1>DevOps 链接导航</h1>
        </div>
        <UserMenu
          user={user}
          onProfile={() => setProfileModalOpen(true)}
          onLogout={logout}
        />
      </header>

      <nav className="main-tabs">
        <button type="button" className={activeTab === "links" ? "tab active" : "tab"} onClick={() => setActiveTab("links")}>链接导航</button>
        {isSuperAdmin ? (
          <>
            <button type="button" className={activeTab === "users" ? "tab active" : "tab"} onClick={() => setActiveTab("users")}>用户管理</button>
            <button type="button" className={activeTab === "audit" ? "tab active" : "tab"} onClick={() => setActiveTab("audit")}>审计日志</button>
          </>
        ) : null}
      </nav>

      {activeTab === "users" && isSuperAdmin ? (
        <UsersPanel token={token} showToast={showToast} />
      ) : activeTab === "audit" && isSuperAdmin ? (
        <AuditPanel token={token} showToast={showToast} />
      ) : (
        <div className="layout">
          <aside className="sidebar">
            <div className="sidebar-head">
              <h2>分类</h2>
              <button
                type="button"
                className="btn-small"
                disabled={!isAdmin}
                title={!isAdmin ? "无权限" : isSuperAdmin ? "新建大分类" : "请联系超级管理员创建大分类"}
                onClick={openCreateRootCategory}
              >
                + 新建
              </button>
            </div>
            <ul className="category-list">
              {categories.map((cat) => {
                const expanded = expandedCategoryIds.has(cat.id);
                return (
                  <li key={cat.id} className="category-group">
                    <div className="category-row category-row-root">
                      <div className="category-item-wrap">
                        <button
                          type="button"
                          className="category-toggle"
                          aria-label={expanded ? "收拢子分类" : "展开子分类"}
                          title={expanded ? "收拢" : "展开"}
                          onClick={() => toggleCategoryExpand(cat.id)}
                        >
                          {expanded ? "−" : "+"}
                        </button>
                        <button
                          type="button"
                          className={selectedCategory?.id === cat.id ? "category-item active" : "category-item"}
                          onClick={() => selectCategory(cat.id)}
                        >
                          <span className="category-name">{cat.name}</span>
                          <span className="count">{cat.link_count ?? getDisplayLinks(cat).length}</span>
                        </button>
                      </div>
                      {cat.can_manage ? (
                        <div className="category-actions">
                          <button type="button" onClick={() => setCategoryModal({ ...cat, sort_order: cat.sort_order ?? 0, parent_id: 0 })}>
                            编辑
                          </button>
                          <button type="button" className="danger" onClick={() => deleteCategory(cat)}>
                            删除
                          </button>
                        </div>
                      ) : null}
                    </div>
                    {expanded ? (
                      <div className="category-children">
                        {(cat.children || []).map((child) => (
                          <div key={child.id} className="category-row category-row-child">
                            <button
                              type="button"
                              className={selectedCategory?.id === child.id ? "category-item child active" : "category-item child"}
                              onClick={() => selectCategory(child.id, cat.id)}
                            >
                              <span className="category-name">{child.name}</span>
                              <span className="count">{(child.links || []).length}</span>
                            </button>
                            {child.can_manage ? (
                              <div className="category-actions">
                                <button type="button" onClick={() => setCategoryModal({ ...child, sort_order: child.sort_order ?? 0, parent_id: cat.id })}>
                                  编辑
                                </button>
                                <button type="button" className="danger" onClick={() => deleteCategory(child)}>
                                  删除
                                </button>
                              </div>
                            ) : null}
                          </div>
                        ))}
                        {isAdmin ? (
                          <button type="button" className="btn-subcategory" onClick={() => openCreateSubCategory(cat.id)}>
                            + 新建子分类
                          </button>
                        ) : null}
                        {!(cat.children || []).length && !isAdmin ? (
                          <div className="category-empty-child">暂无子分类</div>
                        ) : null}
                      </div>
                    ) : null}
                  </li>
                );
              })}
              {categories.length === 0 ? <li className="empty-tip">暂无分类</li> : null}
            </ul>
          </aside>

          <main className="content">
            {selectedCategory ? (
              <>
                <div className="content-head">
                  <div>
                    <span className="category-type-badge">{categoryTypeLabel(selectedCategory)}</span>
                    <h2>{selectedCategory.name}</h2>
                  </div>
                  {selectedCategory.can_manage ? (
                    <button type="button" onClick={() => openLinkModal()}>
                      + 添加链接
                    </button>
                  ) : null}
                </div>
                <div className="link-grid">
                  {displayLinks.map((link) => (
                    <LinkCard
                      key={link.id}
                      link={link}
                      onEdit={openLinkModal}
                      onDelete={deleteLink}
                    />
                  ))}
                  {displayLinks.length === 0 ? (
                    <div className="empty-panel">该分类下暂无链接</div>
                  ) : null}
                </div>
              </>
            ) : (
              <div className="empty-panel">请先创建分类</div>
            )}
          </main>
        </div>
      )}

      {categoryModal ? (
        <div className="modal-overlay" onClick={() => setCategoryModal(null)}>
          <form className="modal" onClick={(e) => e.stopPropagation()} onSubmit={saveCategory}>
            <h3>
              {categoryModal.id
                ? "编辑分类"
                : categoryModal.parent_id
                  ? "新建子分类"
                  : "新建大分类"}
            </h3>
            <label>
              分类名称 *
              <input
                required
                value={categoryModal.name || ""}
                onChange={(e) => setCategoryModal({ ...categoryModal, name: e.target.value })}
              />
            </label>
            <label>
              排序
              <input
                type="number"
                value={categoryModal.sort_order ?? 0}
                onChange={(e) => setCategoryModal({ ...categoryModal, sort_order: e.target.value })}
              />
            </label>
            <div className="modal-actions">
              <button type="button" className="btn-secondary" onClick={() => setCategoryModal(null)}>
                取消
              </button>
              <button type="submit">保存</button>
            </div>
          </form>
        </div>
      ) : null}

      {linkModal ? (
        <div className="modal-overlay" onClick={() => setLinkModal(false)}>
          <form className="modal modal-wide" onClick={(e) => e.stopPropagation()} onSubmit={saveLink}>
            <h3>{linkForm.id ? "编辑链接" : "添加链接"}</h3>
            <label>
              所属分类 *
              <select
                required
                value={linkForm.category_id}
                onChange={(e) => setLinkForm({ ...linkForm, category_id: e.target.value })}
              >
                <option value="">请选择</option>
                {manageableLinkCategories.map((cat) => (
                  <option key={cat.id} value={cat.id}>
                    {cat.label}
                  </option>
                ))}
              </select>
            </label>
            <label>
              网站名称 *
              <input
                required
                value={linkForm.name}
                onChange={(e) => setLinkForm({ ...linkForm, name: e.target.value })}
              />
            </label>
            <label>
              网站地址 *
              <input
                required
                value={linkForm.url}
                onChange={(e) => setLinkForm({ ...linkForm, url: e.target.value })}
                placeholder="https://example.com"
              />
            </label>
            <label>
              网站图标
              <div className="icon-upload-row">
                {linkForm.icon_url || linkForm.icon_path ? (
                  <div className="icon-preview-wrap">
                    {linkForm.icon_url ? (
                      <img src={linkForm.icon_url} alt="" className="icon-preview" />
                    ) : (
                      <div className="icon-preview icon-preview-fallback" title={linkForm.icon_path} />
                    )}
                    <button
                      type="button"
                      className="icon-preview-remove"
                      title="删除图标"
                      aria-label="删除图标"
                      onClick={clearLinkIcon}
                    >
                      ×
                    </button>
                  </div>
                ) : null}
                <input
                  type="file"
                  accept="image/*,.svg,.ico"
                  disabled={uploading}
                  onChange={(e) => uploadIcon(e.target.files?.[0])}
                />
              </div>
            </label>
            <label>
              维护人
              <input
                value={linkForm.maintainer}
                onChange={(e) => setLinkForm({ ...linkForm, maintainer: e.target.value })}
                placeholder="可选"
              />
            </label>
            <label>
              备注
              <textarea
                rows={3}
                value={linkForm.remark}
                onChange={(e) => setLinkForm({ ...linkForm, remark: e.target.value })}
                placeholder="可选"
              />
            </label>
            <label>
              排序
              <input
                type="number"
                value={linkForm.sort_order}
                onChange={(e) => setLinkForm({ ...linkForm, sort_order: e.target.value })}
              />
            </label>
            <div className="modal-actions">
              <button type="button" className="btn-secondary" onClick={() => setLinkModal(false)}>
                取消
              </button>
              <button type="submit">保存</button>
            </div>
          </form>
        </div>
      ) : null}

      <ProfileModal
        open={profileModalOpen}
        onClose={() => setProfileModalOpen(false)}
        token={token}
        user={user}
        onUpdated={refreshUser}
        onRequireRelogin={logout}
        showToast={showToast}
      />

      <ToastBanner toast={toast} onClose={dismissToast} />
    </div>
  );
}
